const assert = require("node:assert/strict");
const test = require("node:test");

test("URL resource titles remain single files in download previews", async () => {
  const { build_resource_tree } = await import("../src/components/tree.js");
  for (const name of [
    "https://",
    "https://example.com/video/123.mp4?next=/watch#part/1",
    "http://example.com/article/123.html",
    "视频 https://example.com/watch/123.mp4",
  ]) {
    const resource = { name, display_name: `${name}.mp4`, resource_index: 3 };
    const tree = build_resource_tree([resource]);
    assert.equal(tree.children.length, 1);
    assert.equal(tree.children[0].type, "file");
    assert.equal(tree.children[0].name, resource.display_name);
    assert.equal(tree.children[0].resource, resource);
  }
});

test("relative resource paths still share their directories", async () => {
  const { build_resource_tree } = await import("../src/components/tree.js");
  const resources = [
    { name: "chapters/volume1/01.txt" },
    { name: "chapters/volume1/02.txt" },
    { name: "cover.jpg" },
  ];
  const tree = build_resource_tree({ resources });
  assert.equal(tree.children.length, 2);
  assert.equal(tree.children[0].type, "directory");
  assert.equal(tree.children[0].name, "chapters");
  const volume = tree.children[0].children[0];
  assert.equal(volume.name, "volume1");
  assert.deepEqual(volume.children.map((node) => node.name), ["01.txt", "02.txt"]);
  assert.equal(tree.children[1].name, "cover.jpg");
});
