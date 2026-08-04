export function TestDialog(props) {
  return Dialog({ store: props.store }, () => [
    View({ dataset: { t: "shared-dialogtest-test-dialog-This-is-a-dialog-content-area-You-can-put-anything-here-1-text" }, class: "text-sm text-zinc-500" }, [
      "This is a dialog content area. You can put anything here.1",
    ]),
  ]);
}
