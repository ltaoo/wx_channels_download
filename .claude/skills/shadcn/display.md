# 展示组件（无 Core）

源文件均在 `packages/shadcn/src/` 下，纯 props + children，无 store。

## Card

```js
Card({ class: "w-[350px]" }, [
  CardHeader({}, [
    CardTitle({}, ["Title"]),
    CardDescription({}, ["Description text"]),
  ]),
  CardContent({}, [/* content */]),
  CardFooter({ class: "flex justify-between" }, [/* actions */]),
]);
```

子组件：`CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`

## Table

```js
Table({}, [
  TableHeader({}, [
    TableRow({}, [
      TableHead({}, ["Name"]),
      TableHead({}, ["Email"]),
    ]),
  ]),
  TableBody({}, [
    TableRow({}, [
      TableCell({}, ["Alice"]),
      TableCell({}, ["alice@example.com"]),
    ]),
  ]),
]);
```

子组件：`TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell`

## Badge

```js
Badge({ variant: "default" }, ["Tag"]);    // "default" | "secondary" | "destructive" | "outline"
```

## Alert

```js
Alert({ variant: "default" }, [            // "default" | "destructive"
  AlertTitle({}, ["Heads up!"]),
  AlertDescription({}, ["Description"]),
]);
```

## 其他

```js
Separator({ orientation: "horizontal" });  // "horizontal" | "vertical"
Avatar({ src: "url", alt: "name", fallback: "AB" });
Skeleton({ class: "h-4 w-[250px]" });
Progress({ value: ref(60), max: 100 });
ScrollArea({ class: "h-72" }, [/* long content */]);
AspectRatio({ ratio: 16 / 9 }, [/* content */]);
Label({ htmlFor: "input-id" }, ["Label text"]);
```

所有展示组件均接受 `class` prop 用于自定义样式。
