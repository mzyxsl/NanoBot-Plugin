// Package manager bot管理相关
package manager

import (
	"strconv"
	"strings"

	nano "github.com/fumiama/NanoBot"

	ctrl "github.com/FloatTech/zbpctrl"
)

func init() {
	en := nano.Register("manager", &ctrl.Options[*nano.Ctx]{
		DisableOnDefault: false,
		Help: "bot管理相关\n" +
			"- /exposeid",
	})
	en.OnMessageCommand("exposeid").SetBlock(true).
		Handle(func(ctx *nano.Ctx) {
			msg := ""
			if nano.OnlyQQ(ctx) {
				msg = "*报告*\n- 群ID: `" + strconv.FormatInt(int64(ctx.GroupID()), 10) + "`\n- 触发用户ID: `" + strconv.FormatInt(int64(ctx.UserID()), 10) + "`"
				for _, e := range strings.Split(ctx.State["args"].(string), " ") {
					e = strings.TrimSpace(e)
					if e == "" {
						continue
					}
					if strings.HasPrefix(e, "<@!") {
						uid := strings.TrimSuffix(e[3:], ">")
						msg += "\n- 用户: " + e + " ID: `" + uid + "`"
					}
				}
			} else {
				msg = "*报告*\n- 频道ID: `" + ctx.Message.ChannelID + "`"
				for _, e := range strings.Split(ctx.State["args"].(string), " ") {
					e = strings.TrimSpace(e)
					if e == "" {
						continue
					}
					if strings.HasPrefix(e, "<@!") {
						uid := strings.TrimSuffix(e[3:], ">")
						msg += "\n- 用户: " + e + " ID: `" + uid + "`"
					}
				}
			}
			_, _ = ctx.SendPlainMessage(true, msg)
		})
}
