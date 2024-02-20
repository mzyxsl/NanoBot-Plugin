// Package status 服务器状态
package status

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/disintegration/imaging"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	nano "github.com/fumiama/NanoBot"

	"github.com/FloatTech/AnimeAPI/setu"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/gg"
	"github.com/FloatTech/imgfactory"
	"github.com/FloatTech/rendercard"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/img/text"

	"github.com/FloatTech/NanoBot-Plugin/kanban/banner"
)

var (
	boottime   = time.Now()
	bgdata     *[]byte
	bgcount    uintptr
	isday      bool
	lightcolor = [3][4]uint8{{255, 70, 0, 255}, {255, 165, 0, 255}, {145, 240, 145, 255}}
	darkcolor  = [3][4]uint8{{215, 50, 0, 255}, {205, 135, 0, 255}, {115, 200, 115, 255}}
)

func init() { // 插件主体
	engine := nano.Register("status", &ctrl.Options[*nano.Ctx]{
		DisableOnDefault: false,
		Brief:            "自检",
		Help:             "- 查询计算机当前活跃度: [检查身体 | 自检 | 启动自检 | 系统状态]",
	})

	engine.OnMessageFullMatchGroup([]string{"检查身体", "自检", "启动自检", "系统状态"}).SetBlock(true).
		Handle(func(ctx *nano.Ctx) {
			now := time.Now().Hour()
			isday = now > 7 && now < 19
			bot, err := ctx.GetMyInfo()
			if err != nil {
				_, _ = ctx.SendPlainMessage(false, "ERROR: ", err)
				return
			}
			img, err := drawstatus(ctx.State["manager"].(*ctrl.Control[*nano.Ctx]), bot.Avatar, bot.Username)
			if err != nil {
				_, _ = ctx.SendPlainMessage(false, "ERROR: ", err)
				return
			}
			sendimg, err := imgfactory.ToBytes(img)
			if err != nil {
				_, _ = ctx.SendPlainMessage(false, "ERROR: ", err)
				return
			}
			if _, err := ctx.SendImageBytes(sendimg, false); err != nil {
				_, _ = ctx.SendPlainMessage(false, "ERROR: ", err)
			}
		})

}

func drawstatus(m *ctrl.Control[*nano.Ctx], botavartarurl, botname string) (sendimg image.Image, err error) {
	diskstate, err := diskstate()
	if err != nil {
		return
	}
	diskcardh := 40 + (20+50)*len(diskstate) + 40 - 20

	moreinfo, err := moreinfo(m)
	if err != nil {
		return
	}
	moreinfocardh := 30 + (20+32*72/96)*len(moreinfo) + 30 - 20

	basicstate, err := basicstate()
	if err != nil {
		return
	}

	dldata := (*[]byte)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&bgdata))))
	if dldata == (*[]byte)(nil) || uintptr(time.Since(boottime).Hours()/24) >= atomic.LoadUintptr(&bgcount) {
		pic, err1 := setu.DefaultPool.Roll("")
		if err1 != nil {
			return nil, err1
		}
		data, err1 := os.ReadFile(pic)
		if err1 != nil {
			return nil, err1
		}
		atomic.AddUintptr(&bgcount, 1)
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&bgdata)), unsafe.Pointer(&data))
		dldata = &data
	}
	data := *dldata

	back, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return
	}

	data, err = web.GetData(botavartarurl)
	if err != nil {
		return
	}
	avatar, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return
	}
	avatarf := imgfactory.Size(avatar, 200, 200)

	fontbyte, err := file.GetLazyData(text.GlowSansFontFile, nano.Md5File, true)
	if err != nil {
		return
	}

	canvas := gg.NewContext(1280, 70+250+40+380+diskcardh+40+moreinfocardh+40+70)

	bh, bw, ch, cw := float64(back.Bounds().Dy()), float64(back.Bounds().Dx()), float64(canvas.H()), float64(canvas.W())

	if bh/bw < ch/cw {
		back = imgfactory.Size(back, int(bw*ch/bh), int(bh*ch/bh)).Image()
		canvas.DrawImageAnchored(back, canvas.W()/2, canvas.H()/2, 0.5, 0.5)
	} else {
		back = imgfactory.Size(back, int(bw*cw/bw), int(bh*cw/bw)).Image()
		canvas.DrawImage(back, 0, 0)
	}
	var blurback image.Image
	bwg := &sync.WaitGroup{}
	bwg.Add(1)
	go func() {
		defer bwg.Done()
		blurback = imaging.Blur(canvas.Image(), 8)
	}()

	if !isday {
		canvas.SetRGBA255(0, 0, 0, 50)
		canvas.DrawRectangle(0, 0, cw, ch)
		canvas.Fill()
	}

	wg := &sync.WaitGroup{}
	wg.Add(5)

	cardw := canvas.W() - 70 - 70

	titlecardh := 250
	basiccardh := 380

	var titleimg, basicimg, diskimg, moreinfoimg, shadowimg image.Image
	go func() {
		defer wg.Done()
		titlecard := gg.NewContext(cardw, titlecardh)
		bwg.Wait()
		titlecard.DrawImage(blurback, -70, -70)

		titlecard.DrawRoundedRectangle(1, 1, float64(titlecard.W()-1*2), float64(titlecardh-1*2), 16)
		titlecard.SetLineWidth(3)
		titlecard.SetColor(colorswitch(100))
		titlecard.StrokePreserve()
		titlecard.SetColor(colorswitch(140))
		titlecard.Fill()

		titlecard.DrawImage(avatarf.Circle(0).Image(), (titlecardh-avatarf.H())/2, (titlecardh-avatarf.H())/2)

		err = titlecard.ParseFontFace(fontbyte, 72)
		if err != nil {
			return
		}
		fw, _ := titlecard.MeasureString(botname)

		titlecard.SetColor(fontcolorswitch())

		titlecard.DrawStringAnchored(botname, float64(titlecardh)+fw/2, float64(titlecardh)*0.5/2, 0.5, 0.5)

		err = titlecard.ParseFontFace(fontbyte, 24)
		if err != nil {
			return
		}
		titlecard.SetColor(fontcolorswitch())

		titlecard.NewSubPath()
		titlecard.MoveTo(float64(titlecardh), float64(titlecardh)/2)
		titlecard.LineTo(float64(titlecard.W()-titlecardh), float64(titlecardh)/2)
		titlecard.Stroke()

		brt, err := botruntime()
		if err != nil {
			return
		}
		fw, _ = titlecard.MeasureString(brt)

		titlecard.DrawStringAnchored(brt, float64(titlecardh)+fw/2, float64(titlecardh)*(0.5+0.5/2), 0.5, 0.5)

		bs, err := botstatus()
		if err != nil {
			return
		}
		fw, _ = titlecard.MeasureString(bs)

		titlecard.DrawStringAnchored(bs, float64(titlecardh)+fw/2, float64(titlecardh)*(0.5+0.75/2), 0.5, 0.5)
		titleimg = rendercard.Fillet(titlecard.Image(), 16)
	}()
	go func() {
		defer wg.Done()
		basiccard := gg.NewContext(cardw, basiccardh)
		bwg.Wait()
		basiccard.DrawImage(blurback, -70, -70-titlecardh-40)

		basiccard.DrawRoundedRectangle(1, 1, float64(basiccard.W()-1*2), float64(basiccardh-1*2), 16)
		basiccard.SetLineWidth(3)
		basiccard.SetColor(colorswitch(100))
		basiccard.StrokePreserve()
		basiccard.SetColor(colorswitch(140))
		basiccard.Fill()

		bslen := len(basicstate)
		for i, v := range basicstate {
			offset := float64(i) * ((float64(basiccard.W())-200*float64(bslen))/float64(bslen+1) + 200)

			basiccard.SetRGBA255(57, 57, 57, 255)
			if isday {
				basiccard.SetRGBA255(235, 235, 235, 255)
			}
			basiccard.DrawCircle((float64(basiccard.W())-200*float64(bslen))/float64(bslen+1)+200/2+offset, 20+200/2, 100)
			basiccard.Fill()

			colors := darkcolor
			if isday {
				colors = lightcolor
			}

			switch {
			case v.precent > 90:
				basiccard.SetColor(slice2color(colors[0]))
			case v.precent > 70:
				basiccard.SetColor(slice2color(colors[1]))
			default:
				basiccard.SetColor(slice2color(colors[2]))
			}

			basiccard.NewSubPath()
			basiccard.MoveTo((float64(basiccard.W())-200*float64(bslen))/float64(bslen+1)+200/2+offset, 20+200/2)
			basiccard.DrawEllipticalArc((float64(basiccard.W())-200*float64(bslen))/float64(bslen+1)+200/2+offset, 20+200/2, 100, 100, -0.5*math.Pi, -0.5*math.Pi+2*v.precent*0.01*math.Pi)
			basiccard.Fill()

			basiccard.SetColor(colorswitch(255))
			basiccard.DrawCircle((float64(basiccard.W())-200*float64(bslen))/float64(bslen+1)+200/2+offset, 20+200/2, 80)
			basiccard.Fill()

			err = basiccard.ParseFontFace(fontbyte, 42)
			if err != nil {
				return
			}

			basiccard.SetRGBA255(213, 213, 213, 255)
			basiccard.DrawStringAnchored(strconv.FormatFloat(v.precent, 'f', 0, 64)+"%", (float64(basiccard.W())-200*float64(bslen))/float64(bslen+1)+200/2+offset, 20+200/2, 0.5, 0.5)

			basiccard.SetColor(fontcolorswitch())

			_, fw := basiccard.MeasureString(v.name)
			basiccard.DrawStringAnchored(v.name, (float64(basiccard.W())-200*float64(bslen))/float64(bslen+1)+200/2+offset, 20+200+15+basiccard.FontHeight()/2, 0.5, 0.5)

			err = basiccard.ParseFontFace(fontbyte, 20)
			if err != nil {
				return
			}
			basiccard.SetColor(fontcolorswitch())

			textoffsety := basiccard.FontHeight() + 10
			for k, s := range v.text {
				basiccard.DrawStringAnchored(s, (float64(basiccard.W())-200*float64(bslen))/float64(bslen+1)+200/2+offset, 20+200+15+fw+15+basiccard.FontHeight()/2+float64(k)*textoffsety, 0.5, 0.5)
			}
		}
		basicimg = rendercard.Fillet(basiccard.Image(), 16)
	}()
	go func() {
		defer wg.Done()
		diskcard := gg.NewContext(cardw, diskcardh)
		bwg.Wait()
		diskcard.DrawImage(blurback, -70, -70-titlecardh-40-basiccardh-40)

		diskcard.DrawRoundedRectangle(1, 1, float64(diskcard.W()-1*2), float64(diskcardh-1*2), 16)
		diskcard.SetLineWidth(3)
		diskcard.SetColor(colorswitch(100))
		diskcard.StrokePreserve()
		diskcard.SetColor(colorswitch(140))
		diskcard.Fill()

		err = diskcard.ParseFontFace(fontbyte, 32)
		if err != nil {
			return
		}

		dslen := len(diskstate)
		if dslen == 1 {
			diskcard.SetRGBA255(57, 57, 57, 255)
			if isday {
				diskcard.SetRGBA255(192, 192, 192, 255)
			}
			diskcard.DrawRoundedRectangle(40, 40, float64(diskcard.W())-40-100, 50, 12)
			diskcard.ClipPreserve()
			diskcard.Fill()

			colors := darkcolor
			if isday {
				colors = lightcolor
			}

			switch {
			case diskstate[0].precent > 90:
				diskcard.SetColor(slice2color(colors[0]))
			case diskstate[0].precent > 70:
				diskcard.SetColor(slice2color(colors[1]))
			default:
				diskcard.SetColor(slice2color(colors[2]))
			}

			diskcard.DrawRoundedRectangle(40, 40, (float64(diskcard.W())-40-100)*diskstate[0].precent*0.01, 50, 12)
			diskcard.Fill()
			diskcard.ResetClip()

			diskcard.SetColor(fontcolorswitch())

			fw, _ := diskcard.MeasureString(diskstate[0].name)
			fw1, _ := diskcard.MeasureString(diskstate[0].text[0])

			diskcard.DrawStringAnchored(diskstate[0].name, 40+10+fw/2, 40+50/2, 0.5, 0.5)
			diskcard.DrawStringAnchored(diskstate[0].text[0], (float64(diskcard.W())-100-10)-fw1/2, 40+50/2, 0.5, 0.5)
			diskcard.DrawStringAnchored(strconv.FormatFloat(diskstate[0].precent, 'f', 0, 64)+"%", float64(diskcard.W())-100/2, 40+50/2, 0.5, 0.5)
		} else {
			for i, v := range diskstate {
				offset := float64(i)*(50+20) - 20

				diskcard.SetRGBA255(57, 57, 57, 255)
				if isday {
					diskcard.SetRGBA255(192, 192, 192, 255)
				}

				diskcard.DrawRoundedRectangle(40, 40+(float64(diskcardh-40*2)-50*float64(dslen))/float64(dslen-1)+offset, float64(diskcard.W())-40-100, 50, 12)
				diskcard.Fill()

				colors := darkcolor
				if isday {
					colors = lightcolor
				}

				switch {
				case v.precent > 90:
					diskcard.SetColor(slice2color(colors[0]))
				case v.precent > 70:
					diskcard.SetColor(slice2color(colors[1]))
				default:
					diskcard.SetColor(slice2color(colors[2]))
				}

				diskcard.DrawRoundedRectangle(40, 40+(float64(diskcardh-40*2)-50*float64(dslen))/float64(dslen-1)+offset, (float64(diskcard.W())-40-100)*v.precent*0.01, 50, 12)
				diskcard.Fill()

				diskcard.SetColor(fontcolorswitch())

				fw, _ := diskcard.MeasureString(v.name)
				fw1, _ := diskcard.MeasureString(v.text[0])

				diskcard.DrawStringAnchored(v.name, 40+10+fw/2, 40+(float64(diskcardh-40*2)-50*float64(dslen))/float64(dslen-1)+50/2+offset, 0.5, 0.5)
				diskcard.DrawStringAnchored(v.text[0], (float64(diskcard.W())-100-10)-fw1/2, 40+(float64(diskcardh-40*2)-50*float64(dslen))/float64(dslen-1)+50/2+offset, 0.5, 0.5)
				diskcard.DrawStringAnchored(strconv.FormatFloat(v.precent, 'f', 0, 64)+"%", float64(diskcard.W())-100/2, 40+(float64(diskcardh-40*2)-50*float64(dslen))/float64(dslen-1)+50/2+offset, 0.5, 0.5)
			}
		}
		diskimg = rendercard.Fillet(diskcard.Image(), 16)
	}()
	go func() {
		defer wg.Done()
		moreinfocard := gg.NewContext(cardw, moreinfocardh)
		bwg.Wait()
		moreinfocard.DrawImage(blurback, -70, -70-titlecardh-40-basiccardh-40-diskcardh-40)

		moreinfocard.DrawRoundedRectangle(1, 1, float64(moreinfocard.W()-1*2), float64(moreinfocard.H()-1*2), 16)
		moreinfocard.SetLineWidth(3)
		moreinfocard.SetColor(colorswitch(100))
		moreinfocard.StrokePreserve()
		moreinfocard.SetColor(colorswitch(140))
		moreinfocard.Fill()

		err = moreinfocard.ParseFontFace(fontbyte, 32)
		if err != nil {
			return
		}

		milen := len(moreinfo)
		for i, v := range moreinfo {
			offset := float64(i)*(20+moreinfocard.FontHeight()) - 20

			moreinfocard.SetColor(fontcolorswitch())

			fw, _ := moreinfocard.MeasureString(v.name)
			fw1, _ := moreinfocard.MeasureString(v.text[0])

			moreinfocard.DrawStringAnchored(v.name, 20+fw/2, 30+(float64(moreinfocardh-30*2)-moreinfocard.FontHeight()*float64(milen))/float64(milen-1)+moreinfocard.FontHeight()/2+offset, 0.5, 0.5)
			moreinfocard.DrawStringAnchored(v.text[0], float64(moreinfocard.W())-20-fw1/2, 30+(float64(moreinfocardh-30*2)-moreinfocard.FontHeight()*float64(milen))/float64(milen-1)+moreinfocard.FontHeight()/2+offset, 0.5, 0.5)
		}
		moreinfoimg = rendercard.Fillet(moreinfocard.Image(), 16)
	}()
	go func() {
		defer wg.Done()
		shadow := gg.NewContext(canvas.W(), canvas.H())
		shadow.SetRGBA255(0, 0, 0, 100)
		shadow.SetLineWidth(12)
		shadow.DrawRoundedRectangle(70, 70, float64(cardw), float64(titlecardh), 16)
		shadow.Stroke()
		shadow.DrawRoundedRectangle(70, float64(70+titlecardh+40), float64(cardw), float64(basiccardh), 16)
		shadow.Stroke()
		shadow.DrawRoundedRectangle(70, float64(70+titlecardh+40+basiccardh+40), float64(cardw), float64(diskcardh), 16)
		shadow.Stroke()
		shadow.DrawRoundedRectangle(70, float64(70+titlecardh+40+basiccardh+40+diskcardh+40), float64(cardw), float64(moreinfocardh), 16)
		shadow.Stroke()
		shadowimg = imaging.Blur(shadow.Image(), 24)
	}()

	wg.Wait()
	if shadowimg == nil || titleimg == nil || basicimg == nil || diskimg == nil || moreinfoimg == nil {
		err = errors.New("图片渲染失败")
		return
	}
	canvas.DrawImage(shadowimg, 0, 0)
	canvas.DrawImage(titleimg, 70, 70)
	canvas.DrawImage(basicimg, 70, 70+titlecardh+40)
	canvas.DrawImage(diskimg, 70, 70+titlecardh+40+basiccardh+40)
	canvas.DrawImage(moreinfoimg, 70, 70+titlecardh+40+basiccardh+40+diskcardh+40)

	err = canvas.ParseFontFace(fontbyte, 28)
	if err != nil {
		return
	}
	canvas.SetRGBA255(0, 0, 0, 255)
	canvas.DrawStringAnchored("Created By NanoBot-Plugin "+banner.Version, float64(canvas.W())/2+3, float64(canvas.H())-70/2+3, 0.5, 0.5)
	canvas.SetRGBA255(255, 255, 255, 255)
	canvas.DrawStringAnchored("Created By NanoBot-Plugin "+banner.Version, float64(canvas.W())/2, float64(canvas.H())-70/2, 0.5, 0.5)

	sendimg = canvas.Image()
	return
}

func botruntime() (string, error) {
	hostinfo, err := host.Info()
	if err != nil {
		return "", err
	}
	t := &strings.Builder{}
	t.WriteString("NanoBot-Plugin 已运行 ")
	t.WriteString(strconv.FormatInt((time.Now().Unix()-boottime.Unix())/86400, 10))
	t.WriteString(" 天 ")
	t.WriteString(time.Unix(time.Now().Unix()-boottime.Unix(), 0).UTC().Format("15:04:05"))
	t.WriteString(" | 系统运行 ")
	t.WriteString(strconv.FormatInt(int64(hostinfo.Uptime)/86400, 10))
	t.WriteString(" 天 ")
	t.WriteString(time.Unix(int64(hostinfo.Uptime), 0).UTC().Format("15:04:05"))
	return t.String(), nil
}

func botstatus() (string, error) {
	hostinfo, err := host.Info()
	if err != nil {
		return "", err
	}
	t := &strings.Builder{}
	t.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	t.WriteString(" | Compiled by ")
	t.WriteString(runtime.Version())
	t.WriteString(" | ")
	t.WriteString(cases.Title(language.English).String(hostinfo.OS))
	t.WriteString(" ")
	t.WriteString(runtime.GOARCH)
	return t.String(), nil
}

type status struct {
	precent float64
	name    string
	text    []string
}

func basicstate() (stateinfo [3]*status, err error) {
	percent, err := cpu.Percent(time.Second, true)
	if err != nil {
		return
	}
	cpuinfo, err := cpu.Info()
	if err != nil {
		return
	}
	cpucore, err := cpu.Counts(false)
	if err != nil {
		return
	}
	cputhread, err := cpu.Counts(true)
	if err != nil {
		return
	}
	cores := strconv.Itoa(cpucore) + "C" + strconv.Itoa(cputhread) + "T"
	times := "最大 " + strconv.FormatFloat(cpuinfo[0].Mhz/1000, 'f', 1, 64) + "Ghz"

	stateinfo[0] = &status{
		precent: math.Round(percent[0]),
		name:    "CPU",
		text:    []string{cores, times},
	}

	raminfo, err := mem.VirtualMemory()
	if err != nil {
		return
	}
	total := "总共 " + storagefmt(float64(raminfo.Total))
	used := "已用 " + storagefmt(float64(raminfo.Used))
	free := "剩余 " + storagefmt(float64(raminfo.Free))

	stateinfo[1] = &status{
		precent: math.Round(raminfo.UsedPercent),
		name:    "RAM",
		text:    []string{total, used, free},
	}

	swapinfo, err := mem.SwapMemory()
	if err != nil {
		return
	}
	total = "总共 " + storagefmt(float64(swapinfo.Total))
	used = "已用 " + storagefmt(float64(swapinfo.Used))
	free = "剩余 " + storagefmt(float64(swapinfo.Free))

	stateinfo[2] = &status{
		precent: math.Round(swapinfo.UsedPercent),
		name:    "SWAP",
		text:    []string{total, used, free},
	}
	return
}

func storagefmt(num float64) string {
	if num /= 1024; num < 1 {
		return strconv.FormatFloat(num*1024, 'f', 2, 64) + "B"
	}
	if num /= 1024; num < 1 {
		return strconv.FormatFloat(num*1024, 'f', 2, 64) + "KB"
	}
	if num /= 1024; num < 1 {
		return strconv.FormatFloat(num*1024, 'f', 2, 64) + "MB"
	}
	if num /= 1024; num < 1 {
		return strconv.FormatFloat(num*1024, 'f', 2, 64) + "GB"
	}
	return strconv.FormatFloat(num, 'f', 2, 64) + "TB"
}

func diskstate() (stateinfo []*status, err error) {
	parts, err := disk.Partitions(false)
	if err != nil {
		return
	}
	stateinfo = make([]*status, 0, len(parts))
	for _, v := range parts {
		mp := v.Mountpoint
		if strings.HasPrefix(mp, "/snap/") || strings.HasPrefix(mp, "/apex/") {
			continue
		}
		diskusage, err := disk.Usage(mp)
		if err != nil {
			continue
		}
		stateinfo = append(stateinfo, &status{
			precent: math.Round(diskusage.UsedPercent),
			name:    mp,
			text:    []string{storagefmt(float64(diskusage.Used)) + " / " + storagefmt(float64(diskusage.Total))},
		})
	}
	return stateinfo, nil
}

func moreinfo(m *ctrl.Control[*nano.Ctx]) (stateinfo []*status, err error) {
	var mems runtime.MemStats
	runtime.ReadMemStats(&mems)
	fmtmem := storagefmt(float64(mems.Sys))

	hostinfo, err := host.Info()
	if err != nil {
		return
	}
	cpuinfo, err := cpu.Info()
	if err != nil {
		return
	}

	count := len(m.Manager.M)
	stateinfo = []*status{
		{name: "OS", text: []string{hostinfo.Platform}},
		{name: "CPU", text: []string{strings.TrimSpace(cpuinfo[0].ModelName)}},
		{name: "Version", text: []string{hostinfo.PlatformVersion}},
		{name: "Plugin", text: []string{"共 " + strconv.Itoa(count) + " 个"}},
		{name: "Memory", text: []string{"已用 " + fmtmem}},
	}
	return
}

func colorswitch(a uint8) color.Color {
	if isday {
		return color.NRGBA{255, 255, 255, a}
	}
	return color.NRGBA{0, 0, 0, a}
}

func fontcolorswitch() color.Color {
	if isday {
		return color.NRGBA{30, 30, 30, 255}
	}
	return color.NRGBA{235, 235, 235, 255}
}

func slice2color(c [4]uint8) color.Color {
	return color.NRGBA{c[0], c[1], c[2], c[3]}
}
