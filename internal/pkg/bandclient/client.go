package bandclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type BandAPI struct {
	BandClient      http.Client
	BandURL         string
	WebHookEndpoint string
}

func NewBandClient(bandURL string, webHookEndpoint string) *BandAPI {
	return &BandAPI{
		BandURL:         bandURL,
		WebHookEndpoint: webHookEndpoint,
	}
}

func (b *BandAPI) SendMessage(ctx context.Context, msg any) error {

	const op = "bandclient.SendMessage"

	payload, _ := json.Marshal(msg)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		b.BandURL+b.WebHookEndpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	resp, err := b.BandClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("BODY BAND: ", string(body))

	return nil

}
