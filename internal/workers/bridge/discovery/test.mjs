import assert from "node:assert/strict";
import { is_public_feed_url } from "./public/_worker.js";

assert.equal(is_public_feed_url("https://example.com/feed.xml"), true);
assert.equal(is_public_feed_url("http://127.0.0.1/feed.xml"), false);
assert.equal(is_public_feed_url("http://10.0.0.1/feed.xml"), false);
assert.equal(is_public_feed_url("http://192.168.1.1/feed.xml"), false);
assert.equal(is_public_feed_url("http://[::1]/feed.xml"), false);
assert.equal(is_public_feed_url("file:///etc/passwd"), false);
console.log("discovery worker URL validation: ok");
