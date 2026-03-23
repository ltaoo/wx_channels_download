# ListCore — 分页列表

源文件：`packages/kit/src/list/index.ts`
类型：`packages/kit/src/list/typing.ts`
常量：`packages/kit/src/list/constants.ts`

## 用法

```js
const list$ = new ListCore(fetchListRequest, {
  pageSize: 10,
});

await list$.init();          // 首次加载
await list$.next();          // 下一页
await list$.prev();          // 上一页
await list$.loadMore();      // 加载更多（追加）
await list$.goto(3, 20);    // 跳转到第3页，每页20条
await list$.search({ keyword: "test" });  // 搜索
await list$.reset();         // 重置到第1页
await list$.reload();        // 重新加载当前页
await list$.refresh();       // 刷新
list$.clear();               // 清空
```

## 数据操作

```js
list$.setDataSource(items);
list$.deleteItem(item => item.id === targetId);
list$.modifyItem(item => item.id === targetId ? { ...item, name: "new" } : item);
list$.replaceDataSource(newItems);
```

## 状态

```ts
list$.response → {
  dataSource: T[],
  page: number,
  pageSize: number,
  total: number,
  search: Record<string, any>,
  initial: boolean,
  noMore: boolean,
  loading: boolean,
  empty: boolean,
  error: BizError | null,
}
```

## 事件

```js
list$.onStateChange(fn)
list$.onLoadingChange(fn)
list$.onDataSourceChange(fn)
list$.onDataSourceAdded(fn)     // loadMore 追加时
list$.onBeforeSearch(fn)
list$.onAfterSearch(fn)
list$.onError(fn)
list$.onComplete(fn)
```

## 注意

- 构造函数接收 `RequestCore` 实例作为数据源
- 内置去抖搜索：`list$.searchDebounce`
- 支持 `next_marker` 分页模式
