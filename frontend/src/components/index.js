export function Section(title, children) {
  return View({ dataset: { t: "shared-index-section-title-value-children-value" }, class: classNames(["space-y-3"]) }, [
    View(
      {
        dataset: { t: "shared-index-section-title-value-heading" },
        class: "text-sm font-semibold text-zinc-500 uppercase tracking-wider",
      },
      [title],
    ),
    View({ dataset: { t: "shared-index-section-stack-children-value" }, class: "space-y-4 pl-1" }, children),
  ]);
}

export function Item(label, children) {
  return View({ dataset: { t: "shared-index-item-stack-label-value-children-value" }, class: "space-y-2" }, [
    View({ dataset: { t: "shared-index-item-label-value-text" }, class: "text-sm text-zinc-400" }, [label]),
    View({ dataset: { t: "shared-index-item-row-children-value" }, class: "flex flex-wrap items-center gap-3" }, children),
  ]);
}
