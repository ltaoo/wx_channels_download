# @timeless/utils 工具函数

源文件：`packages/utils/src/`

kit 直接重导出 utils 中的所有工具。

## 常用函数

```js
// 去抖 / 节流
debounce(300, fn)       // 尾缘去抖
throttle(200, fn)       // 前缘节流

// 延迟
await sleep(1000)

// 安全 JSON 解析
const r = parseJSONStr(str)   // → Result<T>

// 查询字符串
qs_parse("?a=1&b=2")         // { a: "1", b: "2" }
qs_stringify({ a: 1, b: 2 }) // "a=1&b=2"

// 数组 diff（按 id 字段）
diff(oldArr, newArr)  // → { has_update, nodes_added, nodes_updated, nodes_removed }

// 数字工具
toFixed(3.14159, 2)     // 3.14
toNumber("42")          // 42
inRange(5, [1, 10])     // true

// 浏览器工具
downloadFile(url, filename)
loadImage(dataUrl)              // → Result<HTMLImageElement>
readFileAsURL(file)             // → Result<string>
readFileAsArrayBuffer(file)     // → Result<ArrayBuffer>

// 字符串工具
padding_zero(5)         // "05"
random_key(8)           // "a3f8k2m9"

// 中文数字
num_to_chinese(123)     // "一百二十三"
chinese_num_to_num("一百二十三")  // 123

// 不可变数组操作
update_arr_item(arr, 2, newItem)  // 返回新数组
remove_arr_item(arr, 2)           // 返回新数组

// 通用
uidFactory()()          // 自增 ID 生成器
```

## 完整源文件路径

| 模块 | 文件 |
|------|------|
| debounce | `packages/utils/src/lodash/debounce.ts` |
| throttle | `packages/utils/src/lodash/throttle.ts` |
| diff | `packages/utils/src/diff.ts` |
| qs_parse/qs_stringify | `packages/utils/src/qs/index.ts` |
| parseJSONStr | `packages/utils/src/json.ts` |
| toNumber/inRange | `packages/utils/src/primitive.ts` |
| downloadFile | `packages/utils/src/download.ts` |
| loadImage/readFileAsURL/... | `packages/utils/src/browser.ts` |
| 其余（sleep/padding_zero/...） | `packages/utils/src/index.ts` |
