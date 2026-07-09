package officialaccount

import (
	"os"
	"testing"
)

func TestExtractOfficialAccountPageVariableFromArticleSamples(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		nickname string
		avatar   string
		biz      string
		username string
		title    string
	}{
		{
			name:     "picture article",
			file:     "../../../article1.html",
			nickname: "日照茶人茶事",
			avatar:   "http://mmbiz.qpic.cn/mmbiz_png/SZnrAVfCkic3OFY6yqkdN4ib5KAjdva8YPo2T7bjN6EMDdopCEZ158KK6sD4lft9Yd7LUVWicYcIdquH7d3HdCI1g/0?wx_fmt=png",
			biz:      "Mzg3MDYyMTIyNQ==",
			username: "gh_8951dcd584fe",
			title:    "凤求凰",
		},
		{
			name:     "normal article",
			file:     "../../../article2.html",
			nickname: "开智学堂",
			avatar:   "http://mmbiz.qpic.cn/sz_mmbiz_png/jkYHKajOZUpHwV1xSHkeK4Zyx6LyouHWOZYcWLKgyWQSKrC6B5Bz5osRa6FV7yBicaTEQCvfcxCtl2suNr3XZjg/0?wx_fmt=png",
			biz:      "MzkzNDY0MzE1Nw==",
			username: "gh_990683e05e57",
			title:    "阳志平：让 AI 替你自主干活的 12 个技巧",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			if err != nil {
				if os.IsNotExist(err) {
					t.Skipf("%s not found", tt.file)
				}
				t.Fatal(err)
			}
			got := extractOfficialAccountPageVariable(string(data))
			if got.Publisher.Nickname != tt.nickname {
				t.Fatalf("nickname = %q, want %q", got.Publisher.Nickname, tt.nickname)
			}
			if got.Publisher.AvatarURL != tt.avatar {
				t.Fatalf("avatar = %q, want %q", got.Publisher.AvatarURL, tt.avatar)
			}
			if got.Publisher.Biz != tt.biz {
				t.Fatalf("biz = %q, want %q", got.Publisher.Biz, tt.biz)
			}
			if got.Publisher.Username != tt.username {
				t.Fatalf("username = %q, want %q", got.Publisher.Username, tt.username)
			}
			if got.Article.Title != tt.title {
				t.Fatalf("title = %q, want %q", got.Article.Title, tt.title)
			}
		})
	}
}

func TestExtractOfficialAccountPageVariable(t *testing.T) {
	page := extractOfficialAccountPageVariable(`
		<script>
		window.cgiDataNew = {
			title: '主标题\x26amp;副标题',
			nick_name: htmlDecode("作者&nbsp;昵称"),
			round_head_img: '',
			ori_head_img_url: 'https://example.com/132',
			hd_head_img: 'https://example.com/0',
			bizuin: 'MzAwMDA=',
			user_name: 'gh_xxx',
			copyright_info: {
				title: '不能取嵌套标题',
			},
		};
		</script>
	`)
	if page.Article.Title != "主标题&副标题" {
		t.Fatalf("title = %q", page.Article.Title)
	}
	if page.Publisher.Nickname != "作者 昵称" {
		t.Fatalf("nickname = %q", page.Publisher.Nickname)
	}
	if page.Publisher.AvatarURL != "https://example.com/132" {
		t.Fatalf("avatar = %q", page.Publisher.AvatarURL)
	}
	if page.Publisher.Biz != "MzAwMDA=" {
		t.Fatalf("biz = %q", page.Publisher.Biz)
	}
	if page.Publisher.Username != "gh_xxx" {
		t.Fatalf("username = %q", page.Publisher.Username)
	}
}

func TestDecodeWechatJSString(t *testing.T) {
	got := decodeWechatJSString(`'Claude&nbsp;Code\x26amp;AI\nok'`)
	want := "Claude Code&AI\nok"
	if got != want {
		t.Fatalf("decodeWechatJSString() = %q, want %q", got, want)
	}
}
