// Tests adapted from dramadl; see ../NOTICE and ../LICENSE.
package hongguo

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSM3(t *testing.T) {
	for input, expected := range map[string]string{
		"abc":                      "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0",
		strings.Repeat("abcd", 16): "debe9ff92275b8a138604889c18e5a4d6fdb70e5387e5765293dcba39c0c5732",
		strings.Repeat("x", 10000): "335de0cbf893f7af9a3e548067c72f47630b9fa9293d93959c79c2085fab7e5e",
	} {
		if got := hex.EncodeToString(sm3([]byte(input))); got != expected {
			t.Fatalf("SM3 mismatch: %s", got)
		}
	}
}

type signing_fixture struct {
	Now          int64             `json:"now"`
	Nonce        string            `json:"nonce"`
	Payload      string            `json:"payload"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers"`
	Spade        string            `json:"spade"`
	ContentKey   string            `json:"content_key"`
	EncryptedURL string            `json:"encrypted_url"`
	URLSeed      string            `json:"url_seed"`
	MediaURL     string            `json:"media_url"`
}

func load_signing_fixture(t *testing.T) signing_fixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/signing.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture signing_fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestPlaybackSignatureMatchesReference(t *testing.T) {
	fixture := load_signing_fixture(t)
	params := signature_params{nonce: fixture.Nonce, started: fixture.Now, gorgon_random: 16384, helios_random: 1073741824,
		message_random: 1073741824, env_launch: 105, env_pid: 10501, medusa_random: 1073741824, xm_random: 1073741824}
	identity := device_identity{device_id: "7000000000000000001", iid: "7000000000000000002", cdid: "00000000-0000-4000-8000-000000000003"}
	req, err := playback_request([]byte(fixture.Payload), identity, "api5-normal-sinfonlineb.fqnovel.com", time.UnixMilli(fixture.Now), params)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != fixture.URL {
		t.Fatalf("query mismatch\ngot %s\nwant %s", req.URL, fixture.URL)
	}
	for key, expected := range fixture.Headers {
		if got := req.Header.Get(key); got != expected {
			t.Errorf("%s mismatch\ngot  %s\nwant %s", key, got, expected)
		}
	}
}

func TestSignatureBranchesAndAESVariants(t *testing.T) {
	fixture := load_signing_fixture(t)
	raw, err := os.ReadFile("testdata/signature-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Now      int64             `json:"now"`
		Fraction float64           `json:"fraction"`
		URL      string            `json:"url"`
		Headers  map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	identity := device_identity{device_id: "7000000000000000001", iid: "7000000000000000002", cdid: "00000000-0000-4000-8000-000000000003"}
	for _, example := range cases {
		random := uint32(math.Floor(example.Fraction * 4294967296))
		params := signature_params{nonce: fixture.Nonce, started: fixture.Now, gorgon_random: uint16(math.Floor(example.Fraction * 65536)), helios_random: random, message_random: random, env_launch: 100 + uint32(math.Floor(example.Fraction*21)), env_pid: 10001 + uint32(math.Floor(example.Fraction*2000)), medusa_random: random, xm_random: random}
		req, err := playback_request([]byte(fixture.Payload), identity, "api5-normal-sinfonlineb.fqnovel.com", time.UnixMilli(example.Now), params)
		if err != nil {
			t.Fatal(err)
		}
		if req.URL.String() != example.URL {
			t.Fatalf("query mismatch at %d", example.Now)
		}
		for key, expected := range example.Headers {
			if req.Header.Get(key) != expected {
				t.Errorf("%s differs from reference at %d", key, example.Now)
			}
		}
	}
}
