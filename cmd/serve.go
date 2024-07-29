package cmd

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/screego/server/auth"
	"github.com/screego/server/config"
	"github.com/screego/server/logger"
	"github.com/screego/server/router"
	"github.com/screego/server/server"
	"github.com/screego/server/turn"
	"github.com/screego/server/ws"
	"github.com/urfave/cli"
)

func serveCmd(version string) cli.Command {
	return cli.Command{
		Name: "serve",
		Action: func(ctx *cli.Context) {
			// 获取配置
			conf, errs := config.Get()
			// 初始化日志
			logger.Init(conf.LogLevel.AsZeroLogLevel())

			// 处理配置信息
			exit := false
			// 遍历 err 并记录日志，如果有致命错误或恐慌级别的错误，设置 exit 为 true 并退出程序
			for _, err := range errs {
				log.WithLevel(err.Level).Msg(err.Msg)
				exit = exit || err.Level == zerolog.FatalLevel || err.Level == zerolog.PanicLevel
			}
			if exit {
				os.Exit(1)
			}

			// 检查 TURN IP 提供
			if _, _, err := conf.TurnIPProvider.Get(); err != nil {
				// error is already logged by .Get()
				os.Exit(1)
			}

			// 读取用户文件
			users, err := auth.ReadPasswordsFile(conf.UsersFile, conf.Secret, conf.SessionTimeoutSeconds)
			if err != nil {
				log.Fatal().Str("file", conf.UsersFile).Err(err).Msg("While loading users file")
			}

			// 启动 TURN 服务器
			auth, err := turn.Start(conf)
			if err != nil {
				log.Fatal().Err(err).Msg("could not start turn server")
			}

			// 创建和启动房间管理
			rooms := ws.NewRooms(auth, users, conf)

			go rooms.Start()

			// 启动 http 服务器
			r := router.Router(conf, rooms, users, version)
			if err := server.Start(r, conf.ServerAddress, conf.TLSCertFile, conf.TLSKeyFile); err != nil {
				log.Fatal().Err(err).Msg("http server")
			}
		},
	}
}
