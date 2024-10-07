package telegramclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var baseUrl = "https://api.telegram.org/bot%s/%s"

type TGClient struct {
	Token  string
	Client http.Client
}

func NewTGClient(token string) TGClient {
	return TGClient{
		Token: token,
	}
}

func (tg *TGClient) GetMe() {
	urlParam := "getMe"
	url := fmt.Sprintf(baseUrl, tg.Token, urlParam)
	req, _ := http.NewRequest(http.MethodGet, url, nil)

	resp, err := tg.Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Println("tgRespBody: ", string(respBody))
}

func (tg *TGClient) NewMessage(text string) *SendMessageConfig {
	return &SendMessageConfig{
		ChatID:    5568734459,
		Text:      text,
		ParseMode: "MarkdownV2",
	}
}

func (tg *TGClient) SendMessage(c *SendMessageConfig) {
	const urlParam = "sendMessage"
	msgData, _ := json.Marshal(c)

	url := fmt.Sprintf(baseUrl, tg.Token, urlParam)
	fmt.Println("URLURLURL", url)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(msgData))
	req.Header.Add("Content-Type", "application/json")

	resp, err := tg.Client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	//respBody, _ := io.ReadAll(resp.Body)
	//fmt.Println("tgRespBody: ", string(respBody))
}
