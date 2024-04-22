package tgServer

import (
	"bufio"
	"context"
	"fmt"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"os"
	"strconv"
	"strings"
)

type Server struct {
	Db *sqlx.DB
}

func New(db *sqlx.DB) *Server {
	return &Server{
		Db: db,
	}
}

func (s *Server) Start() {
	appId, err := strconv.Atoi(utils.GetEnv("TELEGRAM_APP_ID", ""))
	if err != nil {
		panic(err)
	}
	appHash := utils.GetEnv("TELEGRAM_APP_HASH", "")
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	client := telegram.NewClient(appId, appHash, telegram.Options{
		SessionStorage: NewDbSession(s.Db, 1),
		Logger:         log,
	})
	if err := client.Run(context.Background(), func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			phone := utils.GetEnv("TELEGRAM_PHONE", "")
			password := utils.GetEnv("TELEGRAM_PASSWORD", "")
			codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
				fmt.Print("Enter code: ")
				code, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(code), nil
			}
			if err := auth.NewFlow(
				auth.Constant(phone, password, auth.CodeAuthenticatorFunc(codePrompt)),
				auth.SendCodeOptions{},
			).Run(ctx, client.Auth()); err != nil {
				panic(err)
			}
		}

		api := client.API()
		messages, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID: 0,
			},
		})
		if err != nil {
			return err
		}
		buf := &bin.Buffer{}
		if err := messages.Decode(buf); err != nil {
			return err
		}
		fmt.Println(string(buf.Raw()))
		fmt.Println(messages.String())

		return nil
	}); err != nil {
		panic(err)
	}
}
