package hermes

import (
	"testing"

	"wx_channel/pkg/dm"
)

func TestHookCryptoHmac(t *testing.T) {
	hooks := NewHookManager()
	if err := hooks.Load(`import {createHmac} from "node:crypto";
export function onTaskSuccess() {
  const value = createHmac("sha1", "key").update("da").update(Buffer.from("ta"));
  if (value.digest("hex") !== "104152c5bfdca07bc633eebd46199f0255c9f49d") throw new Error("incorrect HMAC-SHA1");
  if (value.digest("base64") !== "") throw new Error("digest must not be repeated");
  try { value.update("data"); throw new Error("finalized update allowed"); }
  catch (error) { if (error.code !== "ERR_CRYPTO_HASH_FINALIZED") throw error; }
  if (createHmac("sha256", "key").update("data").digest("hex") !== "5031fe3d989c6d1537a013fa6e739da23463fdaec3b70137d828e36ace221bd0") throw new Error("incorrect HMAC-SHA256");
  if (!Buffer.isBuffer(createHmac("sha512", Buffer.from("key")).update("data").digest())) throw new Error("digest must return Buffer");
}`); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
}
