export function Section(title, children) {
  return View({ class: classNames(["space-y-3"]) }, [
    View(
      {
        class: "text-sm font-semibold text-zinc-500 uppercase tracking-wider",
      },
      [title],
    ),
    View({ class: "space-y-4 pl-1" }, children),
  ]);
}

export function Item(label, children) {
  return View({ class: "space-y-2" }, [
    View({ class: "text-sm text-zinc-400" }, [label]),
    View({ class: "flex flex-wrap items-center gap-3" }, children),
  ]);
}
