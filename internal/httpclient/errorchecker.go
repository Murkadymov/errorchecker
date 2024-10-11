package httpclient

import (
	"context"
	"crypto/tls"
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

	"errorchecker/internal/entity/errorchecker"
	"errorchecker/internal/pkg/bandclient"
	"errorchecker/internal/pkg/telegramclient"
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

	checkTableListBody string

	*HTTPClient // dont do
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
		checkTableListBody: `{"sort":[{"columnID":11,"order":"desc"}],"filter":{"search":"","hasPhotoTags":0},"cursor":{"n":20}}`,
		HTTPClient: &HTTPClient{
			client: http.Client{
				Timeout: 10 * time.Second,
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

func (h *ErrorChecker) RunRequests(ctx context.Context, interval int, headers *errorchecker.HeadersStorage, log *slog.Logger) {

	tickerTableList := time.NewTicker(time.Duration(interval) * time.Second)
	tickerGetImt := time.NewTicker(time.Duration(interval) * time.Second)

	h.BandAPI.SendMessage(ctx, bandclient.TextLine{ //TODO: err
		Text: "**WB System Alerter has started...**",
	})

	for {
		select {
		case <-tickerTableList.C:
			err := h.checkTableList(ctx, headers, log)
			if err != nil {
				log.Error("Error executing CheckTableList", "error", err, "handler", "CheckGetImt")
				return
			}
		case <-tickerGetImt.C:
			func() {
				ctxTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
				defer cancel()

				err := h.CheckGetImt(ctxTimeout, headers, log)
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

func makeRequest(ctx context.Context, headers http.Header, method, endpoint, payload string) (int, []byte, error) {
	req, err := http.NewRequest(method, endpoint, strings.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	if req.Body != nil {
		defer req.Body.Close()
	}

	for key, value := range headers {
		for _, v := range value {
			req.Header.Add(key, v)
		}
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, data, nil
}

func (h *ErrorChecker) checkTableList(ctx context.Context, headers *errorchecker.HeadersStorage, log *slog.Logger) error {
	const op = "httpсlient.CheckTableList"

	var header http.Header
	for key, value := range headers.HeadersMap {
		header.Add(key, value)
	}

	for _, cluster := range h.addr {
		endpoint := fmt.Sprintf("%s.%s.%s", h.host, cluster, tableListV6EndPoint)
		statusCode, body, err := makeRequest(ctx, header, http.MethodPost, endpoint, h.checkTableListBody)
		if err != nil {
			// handler error
		}

		switch statusCode {
		case http.StatusOK:
			log.Info(
				"successful request",
				slog.Bool("OK", true),
				slog.String("op", op),
				slog.String("cluster", cluster),
				slog.String("status", strconv.Itoa(statusCode)),
			)

			//msg := bandclient.NewOKMsg(
			//	"worker debug",
			//	resp.Status,
			//	tableListV6EndPoint,
			//	cluster,
			//	formattedTime,
			//	"Skipped",
			//)
			//msg.SetLevel("standard")
			//
			//err = h.BandAPI.SendMessage(ctx, msg)
			//if err != nil {
			//	return fmt.Errorf("%s: %w", op, err)
			//}
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusServiceUnavailable:

			log.Error(
				"request failed",
				slog.Bool("OK", false),
				slog.String("op", op),
				slog.String("cluster", cluster),
				slog.String("status", strconv.Itoa(statusCode)),
			)

			formattedTime := time.Now().Format("2006-01-02 15:04:05")
			msg := bandclient.NewErrMsg(
				mentionMembers,
				statusCode, // TODO: change string to int
				tableListV6EndPoint,
				strings.Trim(strings.Trim(cluster, "."), "/"),
				formattedTime,
				fmt.Sprintf("`%s`", body),
			)
			err := h.BandAPI.SendMessage(ctx, msg)
			if err != nil {
				return fmt.Errorf("%s: %w", err)
			}
		default:
			log.Info(
				"default",
				slog.Int("status", statusCode),
			)
		}
	}
	return nil
}

//func (h *ErrorChecker) CheckTableList(ctx context.Context, headers *errorchecker.HeadersStorage, log *slog.Logger) error {
//	const op = "httpсlient.CheckTableList"
//
//	method := http.MethodPost
//
//	for _, cluster := range h.addr {
//		payload := strings.NewReader(`{"sort":[{"columnID":11,"order":"desc"}],"filter":{"search":"","hasPhotoTags":0},"cursor":{"n":20}}`)
//		endpoint := fmt.Sprintf("%s.%s.%s", h.host, cluster, tableListV6EndPoint)
//		req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
//		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
//			return fmt.Errorf("%s: %w", op, err)
//		}
//
//		for key, value := range headers.HeadersMap {
//			req.Header.Add(key, value)
//		}
//
//		currentTime := time.Now()
//		formattedTime := currentTime.Format("2006\\-01\\-02 15:04:05")
//
//		resp, err := h.client.Do(req)
//		if err != nil {
//			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
//				return fmt.Errorf("%s: %w", op, err)
//			} else {
//				log.Error("ERROR", "error", "error doing checkTableList request")
//				continue
//			}
//
//		}
//
//		defer resp.Body.Close()
//
//		stringBody, err := RespBodyToString(resp)
//		if err != nil {
//			return fmt.Errorf("%s: %w", op, err)
//		}
//
//		switch resp.StatusCode {
//		case http.StatusOK:
//			log.Info(
//				"successful request",
//				slog.Bool("OK", true),
//				slog.String("op", op),
//				slog.String("cluster", cluster),
//				slog.String("status", strconv.Itoa(resp.StatusCode)),
//			)
//
//			//msg := bandclient.NewOKMsg(
//			//	"worker debug",
//			//	resp.Status,
//			//	tableListV6EndPoint,
//			//	cluster,
//			//	formattedTime,
//			//	"Skipped",
//			//)
//			//msg.SetLevel("standard")
//			//
//			//err = h.BandAPI.SendMessage(ctx, msg)
//			//if err != nil {
//			//	return fmt.Errorf("%s: %w", op, err)
//			//}
//		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusServiceUnavailable:
//
//			log.Error(
//				"request failed",
//				slog.Bool("OK", false),
//				slog.String("op", op),
//				slog.String("cluster", cluster),
//				slog.String("status", strconv.Itoa(resp.StatusCode)),
//			)
//
//			msg := bandclient.NewErrMsg(
//				mentionMembers,
//				resp.Status,
//				tableListV6EndPoint,
//				strings.Trim(strings.Trim(cluster, "."), "/"),
//				formattedTime,
//				fmt.Sprintf("`%s`", stringBody),
//			)
//			err := h.BandAPI.SendMessage(ctx, msg)
//			if err != nil {
//				return fmt.Errorf("%s: %w", err)
//			}
//		default:
//			log.Info(
//				"default",
//				slog.String("status", resp.Status),
//			)
//		}
//	}
//	return nil
//}

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
				log.Error("ERROR", "error", "error doing checkTableList request")
				continue
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
				slog.String("cluster", cluster),
				slog.String("status", strconv.Itoa(resp.StatusCode)),
			)

			//msg := bandclient.NewOKMsg(
			//	"worker debug", resp.Status, getImtEndPoint, strings.Trim(strings.Trim(cluster, "."), "/"), formattedTime, "Skipped",
			//)
			//msg.SetLevel("standard")
			//
			//err := h.BandAPI.SendMessage(ctx, msg)
			//if err != nil {
			//	return fmt.Errorf("%s.SendMessage: %w", err)
			//}
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusServiceUnavailable:
			log.Error(
				"successful request",
				slog.Bool("OK", false),
				slog.String("op", op),
				slog.String("status", strconv.Itoa(resp.StatusCode)),
			)

			msg := bandclient.NewErrMsg(
				mentionMembers,
				resp.Status,
				getImtEndPoint,
				strings.Trim(strings.Trim(cluster, "."), "/"),
				formattedTime,
				fmt.Sprintf("`%s`", stringBody),
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
