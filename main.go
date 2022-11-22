package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

var bot *linebot.Client

var botforlog *linebot.Event

type EventType string

// EventType constants

func main() {
	godotenv.Load("local.env")
	var err error
	bot, err = linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_TOKEN"))
	log.Println("Bot:", bot, " err:", err)
	http.HandleFunc("/", callbackHandler)
	port := os.Getenv("PORT")
	addr := fmt.Sprintf(":%s", port)
	http.ListenAndServe(addr, nil)
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	events, err := bot.ParseRequest(r)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			jsonData, err := event.MarshalJSON()
			if err != nil {
				fmt.Printf("could not marshal json: %s\n", err)
				return
			}
			log.Printf("json data is : %s", jsonData)

			switch message := event.Message.(type) {
			// Handle only on text message
			case *linebot.TextMessage:
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("hellow")).Do(); err != nil {
					log.Print(err)
				}
			// Handle only on file massage in group
			case *linebot.FileMessage:
				err := os.MkdirAll("download/file", 0750)
				if err != nil && !os.IsExist(err) {
					log.Fatal(err)
				}
				content, err := bot.GetMessageContent(message.ID).Do()
				if err != nil {
					log.Fatal(err)
				}
				body, err := ioutil.ReadAll(content.Content)
				if err != nil {
					log.Fatal(err)
				}
				defer content.Content.Close()
				err = ioutil.WriteFile("download/file/"+message.FileName, body, 0644)
				// error handling
				if err != nil {
					log.Fatal(err)
				}
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("file already save")).Do(); err != nil {
					log.Print(err)
				}
				// Handle only on file massage in group
			case *linebot.ImageMessage:
				err := os.MkdirAll("download/images", 0750)
				if err != nil && !os.IsExist(err) {
					log.Fatal(err)
				}
				content, err := bot.GetMessageContent(message.ID).Do()
				if err != nil {
					log.Fatal(err)
				}
				body, err := ioutil.ReadAll(content.Content)
				if err != nil {
					log.Fatal(err)
				}
				err = ioutil.WriteFile("download/images/"+message.ID+".jpg", body, 0644)
				// error handling
				if err != nil {
					log.Fatal(err)
				}
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("file already save")).Do(); err != nil {
					log.Print(err)
				}
			// Handle only on Sticker message
			case *linebot.StickerMessage:
				var kw string
				for _, k := range message.Keywords {
					kw = kw + "," + k
				}
				outStickerResult := fmt.Sprintf("收到貼圖訊息: %s, pkg: %s kw: %s  text: %s", message.StickerID, message.PackageID, kw, message.Text)
				if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(outStickerResult)).Do(); err != nil {
					log.Print(err)
				}
			}
		}
	}
}
