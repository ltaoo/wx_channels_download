---
description: "Build pages and use components in the Timeless framework (vanilla JS app)"
---

# Timeless 使用指南

参考 `apps/web-vanilla/src/` 下的页面了解实际用法。

## 全局变量（无需 import）

- `Timeless.ui.*` — Core 类：`ButtonCore`, `InputCore`, `NumberInputCore`, `SelectCore`, `CascaderCore`, `CheckboxCore`, `CheckboxGroupCore`, `RadioGroupCore`, `ToggleCore`, `DialogCore`, `PopoverCore`, `DropdownMenuCore`, `ContextMenuCore`, `MenuCore`, `MenuItemCore`, `TabHeaderCore`, `AccordionCore`, `PresenceCore`, `ToastCore`, `SingleFieldCore`, `ObjectFieldCore`, `DatePickerCore`, `DateRangePickerCore`, `TimePickerCore`
- `Timeless.icons.*` — 图标
- `Timeless.lazy(path)` — 懒加载组件
- `Timeless.buildRoutes(config)` — 构建路由
- `Timeless.Result` — `Result.Ok(v)` / `Result.Err(msg)`
- 响应式：`ref()`, `refobj()`, `refarr()`, `computed()`, `effect()`
- 布局/控制流：`View()`, `Txt()`, `Flex()`, `Show()`, `For()`, `Switch()`, `Match()`, `Fragment()`, `Portal()`, `Transition()`, `Presence()`, `h()`
- UI 组件：`Button()`, `Input()`, `NumberInput()`, `Textarea()`, `Select()`, `Cascader()`, `Checkbox()`, `CheckboxGroup()`, `RadioGroup()`, `Slider()`, `Toggle()`, `DatePicker()`, `DateRangePicker()`, `TimePicker()`, `Label()`, `Badge()`, `Separator()`, `Avatar()`, `Card()`, `CardHeader()`, `CardTitle()`, `CardDescription()`, `CardContent()`, `CardFooter()`, `Table()`, `TableHeader()`, `TableBody()`, `TableRow()`, `TableHead()`, `TableCell()`, `Tabs()`, `Accordion()`, `Steps()`, `Progress()`, `Skeleton()`, `ScrollArea()`, `AspectRatio()`, `Alert()`, `AlertTitle()`, `AlertDescription()`, `Menu()`, `DropdownMenu()`, `ContextMenu()`, `Popover()`, `Popconfirm()`, `Tooltip()`, `TooltipProvider()`, `Sheet()`, `Dialog()`, `Toast()`, `Field()`, `Form()`, `ResizablePanels()`, `ResizablePanel()`, `ResizableHandle()`
- 路由：`RouteSubViews()`, `KeepAliveSubViews()`, `SubViews()`
- 工具：`cn()` — class name 合并

## 响应式 API

```js
const count = ref(0);              // 基本值
const obj = refobj({ a: 1 });     // 响应式对象
const list = refarr([1, 2, 3]);   // 响应式数组

const double = computed(count, (v) => v * 2);  // 派生值
effect(() => { console.log(count.value); });   // 副作用

count.as(5);       // 更新值
list.push(4);      // 数组方法可用
```

## 组件用法

核心模式：创建 `Core` 实例管理状态，传给组件的 `store` prop。

```js
// Button
const btn$ = new Timeless.ui.ButtonCore({ variant: "outline", onClick() {} });
Button({ store: btn$ }, ["Text"]);
btn$.setLoading(true);

// Input / Textarea
Input({ store: new Timeless.ui.InputCore({ defaultValue: "", placeholder: "..." }) });
Textarea({ store: new Timeless.ui.InputCore({ defaultValue: "" }) });

// NumberInput
NumberInput({ store: new Timeless.ui.NumberInputCore({ min: 0, max: 100, step: 5 }), showControls: true });

// Select
Select({ store: new Timeless.ui.SelectCore({ defaultValue: "a", options: [{ value: "a", label: "A" }], onChange(v) {} }) });

// Cascader
Cascader({ store: new Timeless.ui.CascaderCore({ options: [{ value: "a", label: "A", children: [...] }] }) });

// Checkbox / CheckboxGroup / RadioGroup
Checkbox({ store: new Timeless.ui.CheckboxCore({ checked: true }) });
CheckboxGroup({ store: new Timeless.ui.CheckboxGroupCore({ options: [...] }), direction: "horizontal" });
RadioGroup({ store: new Timeless.ui.RadioGroupCore({ value: "a", options: [...] }), direction: "horizontal" });

// Dialog / Sheet
const dialog$ = new Timeless.ui.DialogCore({ title: "Title", footer: true });
Dialog({ store: dialog$ }, [/* content */]);   // dialog$.show() / dialog$.hide()
Sheet({ store: new Timeless.ui.DialogCore({ title: "T" }), side: "right" }, [/* content */]);

// DropdownMenu
DropdownMenu({ store: new Timeless.ui.DropdownMenuCore({
  items: [new Timeless.ui.MenuItemCore({ label: "Edit", onClick() {} })],
}) }, [Button({ store: new Timeless.ui.ButtonCore({}) }, ["Open"])]);

// Popover / Tooltip / Toast
Popover({ store: new Timeless.ui.PopoverCore({ side: "bottom" }), content: [...] }, [/* trigger */]);
TooltipProvider({}, [Tooltip({ content: ["Tip"], side: "top" }, [/* trigger */])]);
const toast$ = new Timeless.ui.ToastCore({});
toast$.show({ texts: ["Done!"] });
Toast({ store: toast$ });

// Tabs / Accordion
Tabs({ store: new Timeless.ui.TabHeaderCore({ selected: "t1", options: [{ label: "T1", value: "t1" }] }) });
Accordion({ store: new Timeless.ui.AccordionCore({ type: "single" }), items: [{ title: "Q", content: "A" }] });

// Transition
const p$ = new Timeless.ui.PresenceCore({});
Transition({ store: p$, animation: { in: "animate-in fade-in", out: "animate-out fade-out" } }, [/* content */]);

// 展示类：Badge, Card, Table, Progress, Skeleton, Alert 等直接传 props
Badge({ variant: "secondary" }, [Txt("Tag")]);
Progress({ value: ref(60), max: 100 });
```

## 控制流

```js
Show({ when: someBoolRef, fallback: [Txt("Loading...")] }, [Txt("Loaded")]);
For({ each: reactiveArray, render(item) { return View({}, [Txt(item.name)]); } });
Switch({ when: computed(store, t => t.type) }, [Match("a", [...]), Match("b", [...])]);
```

## 表单验证

```js
const field$ = new Timeless.ui.SingleFieldCore({
  label: "Email", name: "email",
  input: new Timeless.ui.InputCore({ defaultValue: "", placeholder: "email@example.com" }),
  rules: [{ required: true }],
});
Field({ store: field$ }, [Input({ store: field$.input })]);

const form$ = new Timeless.ui.ObjectFieldCore({ fields: { email: field$ } });
const r = await form$.validate();
if (r.error) { /* handle */ }
const values = r.data;
```

## 路由与导航

```js
// 注册路由（store/index.js）
const routesConfigure = {
  my_page: {
    title: "Page",
    pathname: "/home/my-page",
    component: Timeless.lazy("@/pages/home/my-page.js"),
  },
};

// 导航
props.history.push("root.home_layout.index.form");
props.history.destroyAllAndPush("root.login");

// 侧边栏菜单
Timeless.RouteMenusModel({
  route: props.view.curView?.name,
  history: props.history,
  menus: [{ title: "Home", url: "root.home_layout.index" }],
});

// 布局页使用子视图
KeepAliveSubViews({ ...props });
```

## 页面模板

```js
import { Section, Item } from "@/components/index.js";

export default function MyPageView() {
  return View({ class: "space-y-8" }, [
    Section("Title", [
      Item("Label", [
        // components here
      ]),
    ]),
  ]);
}
```

## 样式

- Tailwind CSS，支持 `dark:` 前缀
- `cn(["flex", "gap-2", condition && "hidden"])` 合并 class
