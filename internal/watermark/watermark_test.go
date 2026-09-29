package watermark

import (
	"strings"
	"testing"
)

func TestImageWatermarkUsesRelativeExpression(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "image", Enabled: true, Path: `C:\logo.png`, X: 0.05, Y: 0.1, WRatio: 0.15, Opacity: 0.8},
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}

	// 位置必须是 ffmpeg 表达式而不是写死的像素，否则改输出分辨率就会跑偏
	if !strings.Contains(res.FilterComplex, "x='w*0.0500'") {
		t.Errorf("X 应转成相对表达式，实际 %q", res.FilterComplex)
	}
	if !strings.Contains(res.FilterComplex, "y='h*0.1000'") {
		t.Errorf("Y 应转成相对表达式，实际 %q", res.FilterComplex)
	}
	// 1920 * 0.15 = 288
	if !strings.Contains(res.FilterComplex, "scale=288:-1") {
		t.Errorf("水印宽度应按目标分辨率换算成像素，实际 %q", res.FilterComplex)
	}
	if !strings.Contains(res.FilterComplex, "colorchannelmixer=aa=0.800") {
		t.Errorf("透明度未生效，实际 %q", res.FilterComplex)
	}
	if len(res.ImageInputs) != 1 || res.ImageInputs[0] != `C:\logo.png` {
		t.Errorf("图片水印应登记为额外输入，实际 %v", res.ImageInputs)
	}
}

func TestMultipleImageWatermarksChainInOrder(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "image", Enabled: true, Path: "a.png", X: 0.1, Y: 0.1, WRatio: 0.1, Opacity: 1},
		{Kind: "image", Enabled: true, Path: "b.png", X: 0.9, Y: 0.9, WRatio: 0.2, Opacity: 1},
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.ImageInputs) != 2 {
		t.Fatalf("应登记两张图片输入，实际 %d", len(res.ImageInputs))
	}
	// 每个图片水印先预处理再叠加，共 2 组
	if n := strings.Count(res.FilterComplex, "overlay="); n != 2 {
		t.Errorf("应有 2 次 overlay，实际 %d：%q", n, res.FilterComplex)
	}
	if n := strings.Count(res.FilterComplex, "colorchannelmixer"); n != 2 {
		t.Errorf("每个图片水印都应处理透明度，实际 %d 次", n)
	}
	// 输入序号必须递增，指向正确的 -i
	if !strings.Contains(res.FilterComplex, "[1:v]") || !strings.Contains(res.FilterComplex, "[2:v]") {
		t.Errorf("图片输入序号不正确：%q", res.FilterComplex)
	}
}

// 图片水印的层叠顺序 = 列表顺序，后面的盖在前面之上
func TestWatermarkOrderIsPreserved(t *testing.T) {
	res, _ := Build([]Item{
		{Kind: "image", Enabled: true, Path: "under.png", X: 0, Y: 0, WRatio: 0.5, Opacity: 1},
		{Kind: "image", Enabled: true, Path: "over.png", X: 0, Y: 0, WRatio: 0.5, Opacity: 1},
	}, 1920, 1080)

	iUnder := strings.Index(res.FilterComplex, "under")
	iOver := strings.Index(res.FilterComplex, "over")
	_ = iUnder
	// 路径本身不出现在表达式里，改用输入序号判断：先登记的先用
	if strings.Index(res.FilterComplex, "[1:v]") > strings.Index(res.FilterComplex, "[2:v]") {
		t.Error("先添加的水印应先被处理")
	}
	_ = iOver
}

func TestTextWatermarkOptions(t *testing.T) {
	res, err := Build([]Item{
		{
			Kind: "text", Enabled: true, Text: "草稿",
			X: 0.5, Y: 0.8, FontSizeRatio: 0.05, Color: "#ffffff", Opacity: 0.9,
			BorderW: 2, BorderColor: "#000000",
			FontFile: `C:\Windows\Fonts\msyh.ttc`,
		},
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}
	fc := res.FilterComplex

	if !strings.Contains(fc, "drawtext=") {
		t.Fatalf("应生成 drawtext，实际 %q", fc)
	}
	// 1080 * 0.05 = 54
	if !strings.Contains(fc, "fontsize=54") {
		t.Errorf("字号应按画面高度换算，实际 %q", fc)
	}
	if !strings.Contains(fc, "fontcolor=0xFFFFFF@0.900") {
		t.Errorf("文字颜色应转成 ffmpeg 格式，实际 %q", fc)
	}
	if !strings.Contains(fc, "borderw=2") || !strings.Contains(fc, "bordercolor=0x000000") {
		t.Errorf("描边参数缺失，实际 %q", fc)
	}
}

// Windows 字体路径里的盘符冒号会被 filter 语法当成参数分隔符，
// 不转义就会报 "No option name near ..." —— drawtext 在 Windows 上最常见的失败原因
func TestWindowsFontPathColonEscaped(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "text", Enabled: true, Text: "x", X: 0, Y: 0, FontSizeRatio: 0.03,
			FontFile: `C:\Windows\Fonts\msyh.ttc`},
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(res.FilterComplex, `fontfile='C\:/Windows/Fonts/msyh.ttc'`) {
		t.Fatalf("字体路径冒号未正确转义，实际 %q", res.FilterComplex)
	}
}

// 文本里的分隔符同样要转义，否则滤镜链会被截断
func TestTextSpecialCharsEscaped(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "text", Enabled: true, Text: "比例 16:9 [测试]", X: 0, Y: 0, FontSizeRatio: 0.03},
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}

	fc := res.FilterComplex
	if strings.Contains(fc, "16:9") {
		t.Errorf("冒号未转义，会被当成参数分隔符：%q", fc)
	}
	if !strings.Contains(fc, `16\:9`) {
		t.Errorf("冒号应转义成 \\:，实际 %q", fc)
	}
	if !strings.Contains(fc, `\[测试\]`) {
		t.Errorf("方括号应转义，实际 %q", fc)
	}
}

func TestTimeRangeGeneratesEnableExpr(t *testing.T) {
	res, _ := Build([]Item{
		{Kind: "text", Enabled: true, Text: "x", X: 0, Y: 0, FontSizeRatio: 0.03,
			HasTime: true, Start: 5, End: 10},
	}, 1000, 1000)

	if !strings.Contains(res.FilterComplex, "enable='between(t,5.000,10.000)'") {
		t.Errorf("时间区间应生成 between 表达式，实际 %q", res.FilterComplex)
	}
}

// 核心行为：每个水印的时间段完全独立，互不影响。
// 三个水印分别是「区间」「全程」「从某点到片尾」，生成的表达式必须逐个对应。
func TestEachWatermarkHasItsOwnTimeRange(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "image", Enabled: true, Path: "a.png", X: 0.05, Y: 0.05, WRatio: 0.2, Opacity: 1,
			HasTime: true, Start: 0, End: 3},
		{Kind: "image", Enabled: true, Path: "b.png", X: 0.8, Y: 0.8, WRatio: 0.2, Opacity: 1,
			HasTime: false}, // 全程
		{Kind: "text", Enabled: true, Text: "尾标", X: 0.1, Y: 0.9, FontSizeRatio: 0.05,
			HasTime: true, Start: 8, End: 0}, // 8 秒起直到片尾
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}

	fc := res.FilterComplex

	// 只有两个水印设了时间段，全程那个不该生成 enable
	if n := strings.Count(fc, "enable="); n != 2 {
		t.Fatalf("应有 2 个时间段限制，实际 %d 个:\n%s", n, fc)
	}
	if !strings.Contains(fc, "enable='between(t,0.000,3.000)'") {
		t.Errorf("第一个水印的区间表达式缺失:\n%s", fc)
	}
	if !strings.Contains(fc, "enable='gte(t,8.000)'") {
		t.Errorf("第三个水印的表达式缺失:\n%s", fc)
	}

	// 逐个链条核对：enable 必须挂在它自己那一步上，不能串到别的步骤。
	// overlay 链按出现顺序即水印顺序。
	chains := strings.Split(fc, ";")
	var overlays []string
	var textChain string
	for _, c := range chains {
		if strings.Contains(c, "overlay=") {
			overlays = append(overlays, c)
		}
		if strings.Contains(c, "drawtext=") {
			textChain = c
		}
	}
	if len(overlays) != 2 {
		t.Fatalf("应有 2 条 overlay 链，实际 %d:\n%s", len(overlays), fc)
	}

	if !strings.Contains(overlays[0], "between(t,0.000,3.000)") {
		t.Errorf("第一个水印的区间限制挂错了位置:\n%s", overlays[0])
	}
	if strings.Contains(overlays[1], "enable=") {
		t.Errorf("第二个水印是全程显示，不该带 enable:\n%s", overlays[1])
	}
	if !strings.Contains(textChain, "gte(t,8.000)") {
		t.Errorf("文字水印的时间限制缺失:\n%s", textChain)
	}
}

// 多个水印各自的时间段不能互相干扰：改一个不该影响另一个
func TestTimeRangesAreIndependentBetweenWatermarks(t *testing.T) {
	items := []Item{
		{Kind: "text", Enabled: true, Text: "A", X: 0.1, Y: 0.1, FontSizeRatio: 0.05, HasTime: true, Start: 1, End: 2},
		{Kind: "text", Enabled: true, Text: "B", X: 0.5, Y: 0.5, FontSizeRatio: 0.05, HasTime: true, Start: 9, End: 12},
	}

	res, err := Build(items, 1000, 1000)
	if err != nil {
		t.Fatal(err)
	}
	fc := res.FilterComplex

	if !strings.Contains(fc, "enable='between(t,1.000,2.000)'") {
		t.Errorf("A 的时间段丢失:\n%s", fc)
	}
	if !strings.Contains(fc, "enable='between(t,9.000,12.000)'") {
		t.Errorf("B 的时间段丢失:\n%s", fc)
	}

	// 混合类型：图片带区间、文字全程、另一张图从某点起
	res2, err := Build([]Item{
		{Kind: "image", Enabled: true, Path: "a.png", X: 0, Y: 0, WRatio: 0.2, Opacity: 1, HasTime: true, Start: 0, End: 5},
		{Kind: "text", Enabled: true, Text: "全程文字", X: 0.5, Y: 0.5, FontSizeRatio: 0.05, HasTime: false},
		{Kind: "image", Enabled: true, Path: "b.png", X: 0.7, Y: 0.7, WRatio: 0.2, Opacity: 1, HasTime: true, Start: 20, End: 0},
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}
	fc2 := res2.FilterComplex

	if n := strings.Count(fc2, "enable="); n != 2 {
		t.Errorf("混合类型时应只有 2 个时间限制，实际 %d:\n%s", n, fc2)
	}
	if !strings.Contains(fc2, "between(t,0.000,5.000)") || !strings.Contains(fc2, "gte(t,20.000)") {
		t.Errorf("图片水印的时间段不正确:\n%s", fc2)
	}
	if !strings.Contains(fc2, "text='全程文字'") {
		t.Errorf("文字水印内容丢失:\n%s", fc2)
	}
}

func TestOpenEndedTimeRange(t *testing.T) {
	res, _ := Build([]Item{
		{Kind: "text", Enabled: true, Text: "x", X: 0, Y: 0, FontSizeRatio: 0.03,
			HasTime: true, Start: 8, End: 0},
	}, 1000, 1000)

	if !strings.Contains(res.FilterComplex, "enable='gte(t,8.000)'") {
		t.Errorf("只给起点应生成 gte 表达式，实际 %q", res.FilterComplex)
	}
}

func TestNoTimeLimitProducesNoEnable(t *testing.T) {
	res, _ := Build([]Item{
		{Kind: "text", Enabled: true, Text: "x", X: 0, Y: 0, FontSizeRatio: 0.03},
	}, 1000, 1000)

	if strings.Contains(res.FilterComplex, "enable=") {
		t.Errorf("全程显示不该出现 enable，实际 %q", res.FilterComplex)
	}
}

// 位置和尺寸越界时不应生成非法表达式
func TestOutOfRangeValuesClamped(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "text", Enabled: true, Text: "x", X: 1.8, Y: -0.5, FontSizeRatio: 0.03, Opacity: 3},
	}, 1000, 1000)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(res.FilterComplex, "x='w*1.8") {
		t.Errorf("横向位置应被夹到 0~1，实际 %q", res.FilterComplex)
	}
	if strings.Contains(res.FilterComplex, "y='h*-0.5") {
		t.Errorf("纵向位置应被夹到 0~1，实际 %q", res.FilterComplex)
	}
	if !strings.Contains(res.FilterComplex, "@1.000") {
		t.Errorf("透明度应被夹到 0~1，实际 %q", res.FilterComplex)
	}
}

func TestDisabledAndEmptyItemsSkipped(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "image", Enabled: false, Path: "a.png"},
		{Kind: "text", Enabled: true, Text: "  "},
		{Kind: "text", Enabled: true, Text: "保留"},
	}, 1000, 1000)
	if err != nil {
		t.Fatal(err)
	}

	if res.Count != 1 {
		t.Errorf("只有 1 个水印应生效，实际 %d", res.Count)
	}
	if len(res.ImageInputs) != 0 {
		t.Error("被禁用的图片水印不该占用输入位")
	}
}

func TestEmptyWatermarkListRejected(t *testing.T) {
	if _, err := Build(nil, 1000, 1000); err == nil {
		t.Fatal("没有水印时应报错")
	}
}

func TestInvalidTargetSizeRejected(t *testing.T) {
	if _, err := Build([]Item{{Kind: "text", Enabled: true, Text: "x"}}, 0, 1080); err == nil {
		t.Fatal("尺寸无效时应报错")
	}
}

// 图片 + 文字混合，验证链条能正确收口到 [vout]
func TestMixedWatermarksProduceSingleOutputLabel(t *testing.T) {
	res, err := Build([]Item{
		{Kind: "image", Enabled: true, Path: "a.png", X: 0.1, Y: 0.1, WRatio: 0.1, Opacity: 1},
		{Kind: "text", Enabled: true, Text: "中间文字", X: 0.4, Y: 0.4, FontSizeRatio: 0.04},
		{Kind: "image", Enabled: true, Path: "b.png", X: 0.9, Y: 0.9, WRatio: 0.05, Opacity: 1},
		{Kind: "text", Enabled: true, Text: "末尾文字", X: 0.1, Y: 0.9, FontSizeRatio: 0.04},
	}, 1920, 1080)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Count(res.FilterComplex, "[vout]") != 1 {
		t.Errorf("[vout] 应只出现一次，实际 %q", res.FilterComplex)
	}
	if !strings.HasSuffix(res.FilterComplex, "[vout]") {
		t.Errorf("[vout] 应在末尾，实际 %q", res.FilterComplex)
	}
	if res.Count != 4 {
		t.Errorf("应生效 4 个水印，实际 %d", res.Count)
	}
	if len(res.ImageInputs) != 2 {
		t.Errorf("应登记 2 张图片，实际 %d", len(res.ImageInputs))
	}
}
