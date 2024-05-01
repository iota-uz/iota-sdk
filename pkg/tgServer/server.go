package tgServer

import (
	"context"
	"fmt"
	"github.com/gotd/td/telegram"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
	"strconv"
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
	//log, err := zap.NewDevelopment()
	//if err != nil {
	//	panic(err)
	//}
	//
	client := telegram.NewClient(appId, appHash, telegram.Options{
		SessionStorage: NewDbSession(s.Db, 1),
		//Logger:         log,
	})
	if err := client.Run(context.Background(), func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return fmt.Errorf("not authorized")
			//phone := utils.GetEnv("TELEGRAM_PHONE", "")
			//password := utils.GetEnv("TELEGRAM_PASSWORD", "")
			//codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
			//	fmt.Print("Enter code: ")
			//	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
			//	if err != nil {
			//		return "", err
			//	}
			//	return strings.TrimSpace(code), nil
			//}
			//if err := auth.NewFlow(
			//	auth.Constant(phone, password, auth.CodeAuthenticatorFunc(codePrompt)),
			//	auth.SendCodeOptions{},
			//).Run(ctx, client.Auth()); err != nil {
			//	panic(err)
			//}
		}

		api := client.API()
		api.MessagesGetChats(ctx, []int64{1})

		//messages, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		//	Channel: &tg.InputChannel{
		//		AccessHash: -1001266437419,
		//		ChannelID:  -1001266437419,
		//	},
		//})
		//if err != nil {
		//	return err
		//}
		//buf := &bin.Buffer{}
		//if err := messages.Decode(buf); err != nil {
		//	return err
		//}
		//fmt.Println(string(buf.Raw()))
		//fmt.Println(messages.String())

		return nil
	}); err != nil {
		panic(err)
	}
}
