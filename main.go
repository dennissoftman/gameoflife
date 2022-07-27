package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc64"
	"image"
	"image/gif"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gameoflife/internal"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

const (
	UpdatesPerMinute = 75
	MaxIterations    = 2048
)

func TerminalUpdate(game *internal.GameOfLife) {
	for {
		fmt.Println(game.Text())
		game.Update()
		time.Sleep(60000 / UpdatesPerMinute * time.Millisecond)
	}
}

func GenerateGIF(game *internal.GameOfLife, output io.Writer) {
	var images []*image.Paletted
	var delays []int

	dataBuffer := make([]byte, game.GetWidth()*game.GetHeight())

	crcTable := crc64.MakeTable(crc64.ISO)
	hash_sums := []uint64{}
	for {
		images = append(images, game.Image(4))
		delays = append(delays, 1)

		game.Update()

		iter := 0
		for i := 0; i < game.GetHeight(); i++ {
			for j := 0; j < game.GetWidth(); j++ {
				if game.At(j, i) {
					dataBuffer[iter] = 64
				} else {
					dataBuffer[iter] = 0
				}
				iter++
			}
		}

		stable := false
		hs := crc64.Checksum(dataBuffer, crcTable)

		for i := 0; i < len(hash_sums); i++ {
			if hash_sums[i] == hs {
				stable = true
				break
			}
		}

		if stable || len(images) == MaxIterations {
			break
		}
		hash_sums = append(hash_sums, hs)
	}

	// write gif
	gif.EncodeAll(output, &gif.GIF{
		Image: images,
		Delay: delays,
	})
}

func PhotoReceivedHandler(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	req := fmt.Sprintf(GetFileUrl, bot.Token, update.Message.Photo[0].FileID)
	resp, err := http.Get(req)
	if err != nil {
		fmt.Printf("Failed to get fileInfo: %v\n", err)
		return
	}
	defer resp.Body.Close()
	// READ JSON
	fileInfoJson, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("unable read img by FileID: %v\n", err)
		return
	}
	// UNMARSHAL JSON
	imgInfo := &ImgFileInfo{}
	err = json.Unmarshal(fileInfoJson, imgInfo)
	if err != nil {
		fmt.Printf("unable unmarshal file description from api.telegram by url: %s, %v\n", req, err)
		return
	}
	// DOWNLOAD IMAGE FILE
	downloadFileUrl := fmt.Sprintf(DownloadFileUrl, bot.Token, imgInfo.Result.FilePath)
	downloadResponse, err := http.Get(downloadFileUrl)
	if err != nil {
		fmt.Printf("unable download file by file_path: %s, %v\n", downloadFileUrl, err)
		return
	}
	defer downloadResponse.Body.Close()

	base_local_path, err := os.MkdirTemp("cache/", "*")
	if err != nil {
		fmt.Printf("failed to create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(base_local_path)
	image_path := filepath.Join(base_local_path, "tmp.jpg")
	fp, err := os.OpenFile(image_path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("failed to open file for writing: %v\n", err)
		return
	}
	defer fp.Close()
	io.Copy(fp, downloadResponse.Body)

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please wait, processing...")
	bot.Send(msg)

	gol, err := internal.LoadFromJPEG(image_path)

	var buffer bytes.Buffer
	GenerateGIF(gol, &buffer)

	final_result := tgbotapi.FileBytes{
		Name:  "result.gif",
		Bytes: buffer.Bytes(),
	}
	anim := tgbotapi.NewAnimation(update.Message.Chat.ID, final_result)
	anim.ReplyToMessageID = update.Message.MessageID
	_, err = bot.Send(anim)
	if err != nil {
		fmt.Printf("failed to send animation: %v\n", err)
		return
	}
}

func TextReceivedHandler(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	data := update.Message.Text

	gol, err := internal.LoadFromText(data, 'o')
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("error while parsing text: %v\n", err))
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
		return
	}

	var buffer bytes.Buffer
	GenerateGIF(gol, &buffer)

	final_result := tgbotapi.FileBytes{
		Name:  "result.gif",
		Bytes: buffer.Bytes(),
	}
	anim := tgbotapi.NewAnimation(update.Message.Chat.ID, final_result)
	anim.ReplyToMessageID = update.Message.MessageID
	_, err = bot.Send(anim)
	if err != nil {
		fmt.Printf("failed to send animation: %v\n", err)
		return
	}
}

const (
	GetFileUrl      = "https://api.telegram.org/bot%s/getFile?file_id=%s"
	DownloadFileUrl = "https://api.telegram.org/file/bot%s/%s"
)

type ImgFileInfo struct {
	Ok     bool `json:"ok"`
	Result struct {
		FileId       string `json:"file_id"`
		FileUniqueId string `json:"file_unique_id"`
		FileSize     int    `json:"file_size"`
		FilePath     string `json:"file_path"`
	} `json:"result"`
}

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Failed to get .env file")
		os.Exit(0)
	}
}

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_API_TOKEN"))
	if err != nil {
		fmt.Printf("Failed to init bot: %v", err)
		return
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if len(update.Message.Photo) > 0 {
			go PhotoReceivedHandler(update, bot)
		} else if update.Message != nil {
			go TextReceivedHandler(update, bot)
		}
	}
}
