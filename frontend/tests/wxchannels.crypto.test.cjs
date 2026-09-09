const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");

async function load_crypto_module() {
  const source = readFileSync(
    path.join(__dirname, "../src/pages/wxchannels.crypto.js"),
    "utf8",
  );
  const module_url = `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
  await import(module_url);
  return globalThis;
}

test("decrypt_wxchannels_data matches Go DecryptData", async () => {
  const encrypted = Uint8Array.from({ length: 32 }, (_value, index) => index);
  const crypto_module = await load_crypto_module();
  crypto_module.decrypt_wxchannels_data(encrypted, 0x3132333435363738n);

  assert.equal(
    Buffer.from(encrypted).toString("hex"),
    "5802e9b727e238c9cb4e1170c6622f288046508747d938b6fe999525150a5138",
  );
});

test("decrypt_wxchannels_data stops at encrypted_length", async () => {
  const encrypted = Uint8Array.from({ length: 40 }, (_value, index) => index);
  const crypto_module = await load_crypto_module();
  crypto_module.decrypt_wxchannels_data(encrypted, 123n, 32);

  assert.deepEqual(
    Array.from(encrypted.slice(32)),
    [32, 33, 34, 35, 36, 37, 38, 39],
  );
});

test("create_wxchannels_decryptor supports Range offsets", async () => {
  const crypto_module = await load_crypto_module();
  const encrypted = Uint8Array.from({ length: 64 }, (_value, index) => index);
  const decrypted = crypto_module.create_wxchannels_decryptor(
    0x3132333435363738n,
    17,
    32,
  ).decrypt_chunk(encrypted);

  assert.equal(
    Buffer.from(decrypted).toString("hex"),
    "57439650c82ba7e1888634021b42290f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f",
  );
});

test("create_wxchannels_decryptor keeps keystream across chunks", async () => {
  const crypto_module = await load_crypto_module();
  const encrypted = Uint8Array.from({ length: 32 }, (_value, index) => index);
  const decryptor = crypto_module.create_wxchannels_decryptor(
    0x3132333435363738n,
    0,
    32,
  );
  let start = 0;
  for (const size of [7, 9, 16]) {
    decryptor.decrypt_chunk(encrypted.subarray(start, start + size));
    start += size;
  }

  assert.equal(
    Buffer.from(encrypted).toString("hex"),
    "5802e9b727e238c9cb4e1170c6622f288046508747d938b6fe999525150a5138",
  );
});
