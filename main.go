package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"linebot-go/configs"
	"linebot-go/model"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

var bot *linebot.Client
var userCollection *mongo.Collection = configs.GetCollection(configs.DB, "users")

// var fileCollection *mongo.Collection = configs.GetCollection(configs.DB, "files")

type EventType string

type Data struct {
	file string
}

func main() {
	godotenv.Load("local.env")
	var err error
	bot, err = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_TOKEN"))
	log.Println("Bot: Connected", " err:", err)
	router := gin.Default()
	data := Data{file: ""}
	router.POST("/", data.callbackHandler)
	router.GET("/download/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		c.Writer.Write(DownloadFile(filename))
		c.JSON(200, gin.H{
			"message": "ok",
		})
	})
	port := ":" + os.Getenv("PORT")
	router.Run(port)
}

func (d *Data) callbackHandler(c *gin.Context) {
	events, err := bot.ParseRequest(c.Request)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
		}
		return
	}
	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			// Handle on print input json massage
			jsonData, err := event.MarshalJSON()
			if err != nil {
				fmt.Printf("could not marshal json: %s\n", err)
				return
			}
			log.Printf("json data is : %s", jsonData)
			switch message := event.Message.(type) {
			// Handle only on text message
			case *linebot.TextMessage:
				Userreply := message.Text
				if Userreply == "ListFile" {
					ListFile := ListItem()
					if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(ListFile)).Do(); err != nil {
						log.Print(err)
					}
				}
				if strings.Contains(Userreply, "Get ") {
					wantfile := strings.TrimPrefix(Userreply, "Get ")
					ListFile := ListItem()
					if strings.Contains(ListFile, wantfile) {
						if strings.Contains(wantfile, "pdf") {
							d.file = wantfile
							if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(strings.Replace("https://4828-203-150-255-17.ap.ngrok.io/download/"+wantfile, " ", "%20", -1))).Do(); err != nil {
								log.Print(err)
							}
						} else if strings.Contains(wantfile, "jpg") {
							d.file = wantfile
							if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewImageMessage("https://4828-203-150-255-17.ap.ngrok.io/download/"+wantfile, "https://4828-203-150-255-17.ap.ngrok.io/download/"+wantfile)).Do(); err != nil {
								log.Print(err)
							}
						}
					}
					if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("File does not exits")).Do(); err != nil {
						log.Print(err)
					}
				}
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("StatusOk")).Do(); err != nil {
					log.Print(err)
				}
			// Handle only on file massage in group
			case *linebot.FileMessage:
				UploadFile("download/file/"+message.FileName, message.FileName)
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("File uploaded to database")).Do(); err != nil {
					log.Print(err)
				}
				// Handle only on Images massage in group
			case *linebot.ImageMessage:
				UploadFile("download/images/"+message.ID+".jpg", message.ID)
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Images uploaded to database")).Do(); err != nil {
					log.Print(err)
				}
			// Handle only on Sticker message
			case *linebot.StickerMessage:
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("StatusOk")).Do(); err != nil {
					log.Print(err)
				}
			}
		}
		if event.Type == linebot.EventTypeFollow {
			jsonData, err := event.MarshalJSON()
			if err != nil {
				fmt.Printf("could not marshal json: %s\n", err)
				return
			}
			log.Printf("json data is : %s", jsonData)
			_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			newUser := model.User{
				Id:        event.Source.UserID,
				CreatedAt: event.Timestamp.Local(),
			}
			_, err = userCollection.InsertOne(context.Background(), newUser)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Collect UserID to Database")).Do(); err != nil {
				log.Print(err)
			}
			c.JSON(http.StatusCreated, gin.H{
				"message": "success",
			})
		}

		if event.Type == linebot.EventTypeUnfollow {
			jsonData, err := event.MarshalJSON()
			if err != nil {
				fmt.Printf("could not marshal json: %s\n", err)
				return
			}
			log.Printf("json data is : %s", jsonData)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result, err := userCollection.DeleteOne(ctx, bson.M{"id": event.Source.UserID})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Remove UserID from Database")).Do(); err != nil {
				log.Print(err)
			}
			fmt.Printf("DeleteOne removed %v document(s)\n", result.DeletedCount)
			c.JSON(http.StatusCreated, gin.H{
				"message": "success",
			})
		}
	}
}

func ListItem() string {
	var Liststring []string
	bucket, err := gridfs.NewBucket(
		configs.ConnectDB().Database("files"),
	)
	if err != nil {
		panic(err)
	}
	filter := bson.D{{}}
	cursor, err := bucket.Find(filter)
	if err != nil {
		panic(err)
	}
	type gridfsFile struct {
		Name string `bson:"filename"`
	}
	var foundFiles []gridfsFile
	if err = cursor.All(context.TODO(), &foundFiles); err != nil {
		panic(err)
	}
	for i, file := range foundFiles {
		if i <= 9 {
			Liststring = append(Liststring, fmt.Sprintf("%d.filename: %s", i+1, file.Name))
		}
	}
	String := strings.Join(Liststring, "\n")
	return String
}

func UploadFile(file, filename string) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	bucket, err := gridfs.NewBucket(
		configs.ConnectDB().Database("files"),
	)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	uploadStream, err := bucket.OpenUploadStream(
		filename,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer uploadStream.Close()
	fileSize, err := uploadStream.Write(data)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	log.Printf("Write file to DB was successful. File size: %d M\n", fileSize)
}

func DownloadFile(fileName string) []byte {
	// For CRUD operations, here is an example
	db := configs.ConnectDB().Database("files")
	fsFiles := db.Collection("fs.files")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	var results bson.M
	err := fsFiles.FindOne(ctx, bson.M{"filename": fileName}).Decode(&results)
	if err != nil {
		log.Fatal(err)
	}
	// you can print out the results
	fmt.Println(results)
	bucket, _ := gridfs.NewBucket(
		db,
	)
	var buf bytes.Buffer
	dStream, err := bucket.DownloadToStreamByName(fileName, &buf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File size to download: %v\n", dStream)
	return buf.Bytes()
}
