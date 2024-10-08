package httpclient

import (
	"context"
	"crypto/tls"
	"errorchecker/internal/entity/errorchecker"
	"errorchecker/internal/pkg/bandclient"
	"errorchecker/internal/pkg/telegramclient"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	mentionMembers      = "@kadymov.murad"
	tableListV6EndPoint = "viewer/viewer/tableListv6"
	getImtEndPoint      = "viewer/viewer/getImt"
)

var temp = template.New(
	"ALERT TEST\nHandler: viewer/tableListv6\nStatusCode: {.status}\nTime: {.time}\n @murad.kadymov")

type ErrorChecker struct {
	TGClient *telegramclient.TGClient
	BandAPI  *bandclient.BandAPI
	*HTTPClient
}

type HTTPClient struct {
	client      http.Client
	host        string
	addr        []string
	stopChannel chan struct{}
	wg          *sync.WaitGroup
}

func NewErrorChecker(host string, cluster []string, token string, bandURL string, webHookEndpoint string) *ErrorChecker {
	return &ErrorChecker{
		BandAPI: bandclient.NewBandClient(bandURL, webHookEndpoint),
		TGClient: &telegramclient.TGClient{
			Token: token,
		},
		HTTPClient: &HTTPClient{
			client: http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			},
			host:        host,
			addr:        cluster,
			stopChannel: make(chan struct{}),
		}}
}

func RespBodyToString(response *http.Response) (string, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("reponse to body converting error: %w", err)
	}

	return string(body), nil
}

func (h *ErrorChecker) StopChecker() {
	h.stopChannel <- struct{}{}
}

func (h *ErrorChecker) RunRequests(ctx context.Context, headers *errorchecker.HeadersStorage, log *slog.Logger, wg *sync.WaitGroup) {
	tickerTableList := time.NewTicker(5 * time.Second)
	tickerGetImt := time.NewTicker(5 * time.Second)
	//h.BandAPI.SendMessage(ctx, bandclient.TextLine{ //TODO: err
	//	Text: "refactor ingress v1, restart",
	//})
	ctxTimeOut, cancel := context.WithTimeout(ctx, 60*time.Second)

	defer cancel()

	for {
		select {
		case <-tickerTableList.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := h.CheckTableList(ctxTimeOut, headers, log)
				if err != nil {
					log.Error("Error executing CheckTableList", "error", err, "handler", "CheckGetImt")
					return
				}
			}()
		case <-tickerGetImt.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := h.CheckGetImt(ctxTimeOut, headers, log)
				if err != nil {
					log.Error("Error executing CheckGetImt", "error", err, "handler", "CheckGetImt")
					return
				}

			}()
		case <-ctx.Done():
			if err := ctx.Err(); errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Error("context deadline exceeded", "error", err)
			}
			if err := ctx.Err(); errors.Is(ctx.Err(), context.Canceled) {
				log.Error("context has been canceled", "error", err)
			}
			log.Info("ended sending requests")

			tickerTableList.Stop()
			tickerGetImt.Stop()

			return
		}

	}

}

func (h *ErrorChecker) CheckTableList(ctx context.Context, headers *errorchecker.HeadersStorage, log *slog.Logger) error {
	const op = "httpсlient.CheckTableList"

	method := http.MethodPost

	for _, cluster := range h.addr {
		payload := strings.NewReader(`{"sort":[{"columnID":11,"order":"desc"}],"filter":{"search":"","hasPhotoTags":0},"cursor":{"n":20}}`)

		req, err := http.NewRequestWithContext(ctx, method, h.host+cluster+tableListV6EndPoint, payload)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%s: %w", op, err)
		}

		for key, value := range headers.HeadersMap {
			req.Header.Add(key, value)
		}

		currentTime := time.Now()
		formattedTime := currentTime.Format("2006\\-01\\-02 15:04:05")

		resp, err := h.client.Do(req)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("%s: %w", op, err)
			} else {
				panic(err)
			}

		}
		fmt.Println("tableList req sent")

		defer resp.Body.Close()

		stringBody, err := RespBodyToString(resp)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			log.Info(
				"successful request",
				slog.Bool("OK", true),
				slog.String("op", op),
				slog.String("cluster", cluster),
				slog.String("status", strconv.Itoa(resp.StatusCode)),
			)

			//msg := bandclient.NewOKMsg(
			//	"worker debug", resp.Status, tableListV6EndPoint, host, formattedTime, "Skipped",
			//)
			//msg.SetLevel("standard")
			//
			//err = h.BandAPI.SendMessage(ctx, msg)
			//if err != nil {
			//	return fmt.Errorf("%s: %w", op, err)
			//}
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusServiceUnavailable:

			log.Info(
				"request failed",
				slog.Bool("OK", false),
				slog.String("op", op),
				slog.String("cluster", cluster),
				slog.String("status", strconv.Itoa(resp.StatusCode)),
			)

			msg := bandclient.NewErrMsg(
				mentionMembers, resp.Status, tableListV6EndPoint, strings.Trim(strings.Trim(cluster, "."), "/"), formattedTime, stringBody,
			)
			err := h.BandAPI.SendMessage(ctx, msg)
			if err != nil {
				return fmt.Errorf("%s: %w", err)
			}
		default:
			log.Info(
				"default",
				slog.String("status", resp.Status),
			)
		}
	}
	return nil
}

func (h *ErrorChecker) CheckGetImt(ctx context.Context, headers *errorchecker.HeadersStorage, log *slog.Logger) error {
	const op = "httpсlient.CheckGetImt"

	method := http.MethodPost

	for _, cluster := range h.addr {
		payload := strings.NewReader(`{"nmID":265938554}`)

		req, err := http.NewRequestWithContext(ctx, method, h.host+cluster+getImtEndPoint, payload)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("%s: %w", op, err)
			}
			return fmt.Errorf("%s: %w", op, err)
		}

		for key, value := range headers.HeadersMap {
			req.Header.Add(key, value)
		}

		currentTime := time.Now()
		formattedTime := currentTime.Format("2006\\-01\\-02 15:04:05")

		resp, err := h.client.Do(req)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("%s: %w", op, err)
			} else {
				panic(err)
			}

		}

		defer resp.Body.Close()

		stringBody, err := RespBodyToString(resp)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			log.Info(
				"successful request",
				slog.Bool("OK", true),
				slog.String("op", op),
				slog.String("status", strconv.Itoa(resp.StatusCode)),
			)

			msg := bandclient.NewOKMsg(
				"worker debug", resp.Status, getImtEndPoint, strings.Trim(strings.Trim(cluster, "."), "/"), formattedTime, "Skipped",
			)
			msg.SetLevel("standard")

			err := h.BandAPI.SendMessage(ctx, msg)
			if err != nil {
				return fmt.Errorf("%s.SendMessage: %w", err)
			}
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusServiceUnavailable:
			log.Info(
				"successful request",
				slog.Bool("OK", false),
				slog.String("op", op),
				slog.String("status", strconv.Itoa(resp.StatusCode)),
			)

			msg := bandclient.NewErrMsg(
				mentionMembers, resp.Status, tableListV6EndPoint, strings.Trim(strings.Trim(cluster, "."), "/"), formattedTime, stringBody,
			)

			err := h.BandAPI.SendMessage(ctx, msg)
			if err != nil {
				return fmt.Errorf("%s.SendMessage: %w", op, err)
			}
		default:
			log.Info(
				"default",
				slog.String("status", resp.Status),
			)
		}
	}
	return nil
}
