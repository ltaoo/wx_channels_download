package officialaccount

import "testing"

func TestParseCgiDataNewAcceptsStringUserUin(t *testing.T) {
	html := `<html><body><script>
window.cgiDataNew = {
  user_uin: "69477998648217",
  title: "test title",
  content_noencode: "<p>body</p>",
  page_type: 1
};
</script></body></html>`

	data, err := parse_cgi_datanew(html)
	if err != nil {
		t.Fatal(err)
	}
	if data.Title != "test title" {
		t.Fatalf("Title = %q, want %q", data.Title, "test title")
	}
}
