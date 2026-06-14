package kernel

import (
	"fmt"
	"strings"
)

// CardElement 是所有卡片元素的公共接口
type CardElement interface {
	cardElement()
}

// CardMarkdown 渲染 markdown 格式文本
type CardMarkdown struct{ Content string }

func (CardMarkdown) cardElement() {}

// CardDivider 渲染水平分隔线
type CardDivider struct{}

func (CardDivider) cardElement() {}

// CardActions 渲染按钮行
type CardActions struct {
	Buttons []CardButton
	Layout  CardActionLayout
}

func (CardActions) cardElement() {}

// CardButton 按钮
type CardButton struct {
	Text  string
	Type  string // "primary", "default", "danger"
	Value string // 回调数据
	Extra map[string]string
}

// CardListItem 左侧文字 + 右侧按钮的布局
type CardListItem struct {
	Text     string
	BtnText  string
	BtnType  string
	BtnValue string
	Extra    map[string]string
}

func (CardListItem) cardElement() {}

// CardSelect 下拉选择器
type CardSelect struct {
	Placeholder string
	Options     []CardSelectOption
	InitValue  string
}

func (CardSelect) cardElement() {}

// CardSelectOption 选择项
type CardSelectOption struct {
	Text  string
	Value string
}

// CardNote 脚注文本
type CardNote struct {
	Text string
	Tag  string // 可选标签，用于程序识别
}

func (CardNote) cardElement() {}

// CardActionLayout 按钮布局模式
type CardActionLayout string

const (
	CardActionLayoutRow          CardActionLayout = "row"
	CardActionLayoutEqualColumns CardActionLayout = "equal_columns"
)

// CardHeader 卡片标题头
type CardHeader struct {
	Title string
	Color string // blue, green, red, orange, purple, grey
}

// Card 代表一个结构化卡片消息
type Card struct {
	Header   *CardHeader
	Elements []CardElement
}

// Btn 是 CardButton 的简写构造函数
func Btn(text, typ, value string) CardButton {
	return CardButton{Text: text, Type: typ, Value: value}
}

// PrimaryBtn 创建主按钮
func PrimaryBtn(text, value string) CardButton {
	return CardButton{Text: text, Type: "primary", Value: value}
}

// DefaultBtn 创建默认按钮
func DefaultBtn(text, value string) CardButton {
	return CardButton{Text: text, Type: "default", Value: value}
}

// DangerBtn 创建危险按钮
func DangerBtn(text, value string) CardButton {
	return CardButton{Text: text, Type: "danger", Value: value}
}

// CardBuilder 链式构建器
type CardBuilder struct {
	card Card
}

// NewCard 返回一个新的 CardBuilder
func NewCard() *CardBuilder {
	return &CardBuilder{}
}

// Title 设置卡片标题头
func (b *CardBuilder) Title(title, color string) *CardBuilder {
	b.card.Header = &CardHeader{Title: title, Color: color}
	return b
}

// Markdown 添加一个 markdown 文本元素
func (b *CardBuilder) Markdown(content string) *CardBuilder {
	if content != "" {
		b.card.Elements = append(b.card.Elements, CardMarkdown{Content: content})
	}
	return b
}

// Markdownf 添加一个格式化的 markdown 文本元素
func (b *CardBuilder) Markdownf(format string, args ...any) *CardBuilder {
	return b.Markdown(fmt.Sprintf(format, args...))
}

// Divider 添加一个水平分隔线
func (b *CardBuilder) Divider() *CardBuilder {
	b.card.Elements = append(b.card.Elements, CardDivider{})
	return b
}

// Buttons 添加一行按钮
func (b *CardBuilder) Buttons(buttons ...CardButton) *CardBuilder {
	if len(buttons) > 0 {
		b.card.Elements = append(b.card.Elements, CardActions{Buttons: buttons, Layout: CardActionLayoutRow})
	}
	return b
}

// ButtonsEqual 添加一行等宽按钮
func (b *CardBuilder) ButtonsEqual(buttons ...CardButton) *CardBuilder {
	if len(buttons) > 0 {
		b.card.Elements = append(b.card.Elements, CardActions{Buttons: buttons, Layout: CardActionLayoutEqualColumns})
	}
	return b
}

// ListItem 添加一个列表项：左侧描述 + 右侧按钮
func (b *CardBuilder) ListItem(desc, btnText, btnValue string) *CardBuilder {
	b.card.Elements = append(b.card.Elements, CardListItem{
		Text: desc, BtnText: btnText, BtnType: "default", BtnValue: btnValue,
	})
	return b
}

// ListItemBtn 添加一个列表项，可指定按钮类型
func (b *CardBuilder) ListItemBtn(desc, btnText, btnType, btnValue string) *CardBuilder {
	b.card.Elements = append(b.card.Elements, CardListItem{
		Text: desc, BtnText: btnText, BtnType: btnType, BtnValue: btnValue,
	})
	return b
}

// ListItemBtnExtra 添加一个带额外数据的列表项
func (b *CardBuilder) ListItemBtnExtra(desc, btnText, btnType, btnValue string, extra map[string]string) *CardBuilder {
	b.card.Elements = append(b.card.Elements, CardListItem{
		Text: desc, BtnText: btnText, BtnType: btnType, BtnValue: btnValue, Extra: extra,
	})
	return b
}

// Select 添加一个下拉选择器
func (b *CardBuilder) Select(placeholder string, options []CardSelectOption, initValue string) *CardBuilder {
	if len(options) > 0 {
		b.card.Elements = append(b.card.Elements, CardSelect{
			Placeholder: placeholder, Options: options, InitValue: initValue,
		})
	}
	return b
}

// Note 添加一个脚注元素
func (b *CardBuilder) Note(text string) *CardBuilder {
	if text != "" {
		b.card.Elements = append(b.card.Elements, CardNote{Text: text})
	}
	return b
}

// TaggedNote 添加一个带标签的脚注
func (b *CardBuilder) TaggedNote(tag, text string) *CardBuilder {
	if text != "" {
		b.card.Elements = append(b.card.Elements, CardNote{Text: text, Tag: tag})
	}
	return b
}

// Build 返回构建好的 Card
func (b *CardBuilder) Build() *Card {
	c := b.card
	return &c
}

// RenderText 将卡片转换为纯文本，用于不支持卡片的平台
// 保留 Markdown 代码块，其他转文本
func (c *Card) RenderText() string {
	var sb strings.Builder

	if c.Header != nil && c.Header.Title != "" {
		sb.WriteString("**")
		sb.WriteString(c.Header.Title)
		sb.WriteString("**\n\n")
	}

	for _, elem := range c.Elements {
		switch e := elem.(type) {
		case CardMarkdown:
			// 保留代码块，其他简化
			content := simplifyMarkdown(e.Content)
			sb.WriteString(content)
			sb.WriteString("\n\n")
		case CardDivider:
			sb.WriteString("---\n\n")
		case CardActions:
			for i, btn := range e.Buttons {
				if i > 0 {
					sb.WriteString("  ")
				}
				sb.WriteString("[")
				sb.WriteString(btn.Text)
				sb.WriteString("]")
			}
			sb.WriteString("\n\n")
		case CardListItem:
			sb.WriteString(e.Text)
			sb.WriteString("  [")
			sb.WriteString(e.BtnText)
			sb.WriteString("]\n")
		case CardSelect:
			sb.WriteString(e.Placeholder)
			sb.WriteString(": ")
			for i, opt := range e.Options {
				if i > 0 {
					sb.WriteString(" | ")
				}
				sb.WriteString(opt.Text)
			}
			sb.WriteString("\n\n")
		case CardNote:
			sb.WriteString(e.Text)
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// simplifyMarkdown 简化 markdown，保留代码块
func simplifyMarkdown(content string) string {
	// 保留代码块（```开头和结尾的）
	// 其他 markdown 符号简化
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// 简化标题
		if strings.HasPrefix(line, "### ") {
			lines[i] = "**" + line[4:] + "**"
		} else if strings.HasPrefix(line, "## ") {
			lines[i] = "**" + line[3:] + "**"
		} else if strings.HasPrefix(line, "# ") {
			lines[i] = "**" + line[2:] + "**"
		}
	}
	return strings.Join(lines, "\n")
}

// HasButtons 返回卡片是否包含交互按钮
func (c *Card) HasButtons() bool {
	for _, elem := range c.Elements {
		switch elem.(type) {
		case CardActions, CardListItem, CardSelect:
			return true
		}
	}
	return false
}
