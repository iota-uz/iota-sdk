package tgserver

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-sdk/pkg/utils/env"
	"strconv"

	"github.com/gotd/td/telegram"
	"github.com/jmoiron/sqlx"
)

type Server struct {
	DB *sqlx.DB
}

func New(db *sqlx.DB) *Server {
	return &Server{
		DB: db,
	}
}

func (s *Server) Start() {
	appID, err := strconv.Atoi(env.GetEnv("TELEGRAM_APP_ID", ""))
	if err != nil {
		panic(err)
	}
	appHash := env.GetEnv("TELEGRAM_APP_HASH", "")
	// log, err := zap.NewDevelopment()
	// if err != nil {
	//	panic(err)
	//}
	//
	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: NewDBSession(s.DB, 1),
		// Logger:         log,
	})
	if err := client.Run(context.Background(), func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return errors.New("not authorized")
			// phone := utils.GetEnv("TELEGRAM_PHONE", "")
			// password := utils.GetEnv("TELEGRAM_PASSWORD", "")
			// codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
			//	fmt.Print("Enter code: ")
			//	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
			//	if err != nil {
			//		return "", err
			//	}
			//	return strings.TrimSpace(code), nil
			//}
			// if err := auth.NewFlow(
			//	auth.Constant(phone, password, auth.CodeAuthenticatorFunc(codePrompt)),
			//	auth.SendCodeOptions{},
			// ).Run(ctx, client.Auth()); err != nil {
			//	panic(err)
			//}
		}

		api := client.API()
		_, err = api.MessagesGetChats(ctx, []int64{1})
		if err != nil {
			return err
		}

		// messages, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		//	Channel: &tg.InputChannel{
		//		AccessHash: -1001266437419,
		//		ChannelID:  -1001266437419,
		//	},
		// })
		// if err != nil {
		//	 return err
		// }
		// buf := &bin.Buffer{}
		// if err := messages.Decode(buf); err != nil {
		//	return err
		// }
		// fmt.Println(string(buf.Raw()))
		// fmt.Println(messages.String())

		return nil
	}); err != nil {
		panic(err)
	}
}
