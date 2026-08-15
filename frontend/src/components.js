function Popover(props, children) {
  const {
    store,
    content,
    onTriggerMouseEnter,
    onTriggerMouseLeave,
    ...content_props
  } = props;
  const presence_state_ = refobj(props.store.presence.state);
  const was_exiting_ = ref(false);
  const unlistens = [
    props.store.presence.onStateChange((state) => {
      presence_state_.as(state);
      if (state.exit) {
        was_exiting_.as(true);
      }
      if (state.mounted) {
        was_exiting_.as(false);
      }
    }),
  ];

  return Timeless.ui.PopoverPrimitive.Root(
    {
      onUnmounted() {
        unlistens.forEach((unlisten) => {
          if (typeof unlisten === "function") {
            unlisten();
          }
        });
      },
    },
    [
      View(
        {
          class: "wx-home-popover-hover-trigger",
          onMounted(event) {
            const trigger_root = event.target;
            const trigger_children =
              typeof trigger_root.getChildren === "function"
                ? trigger_root.getChildren()
                : [];
            const trigger =
              trigger_children.find(
                (child) => child && child.getType() === "view",
              ) ||
              trigger_children[0] ||
              trigger_root;
            store.popper.setReference(
              {
                $el: trigger,
                getRect: () => trigger.getBoundingClientRect(),
              },
              { force: true },
            );
          },
          onMouseEnter(event) {
            if (typeof onTriggerMouseEnter === "function") {
              onTriggerMouseEnter(event);
            }
          },
          onMouseLeave(event) {
            if (typeof onTriggerMouseLeave === "function") {
              onTriggerMouseLeave(event);
            }
          },
        },
        children,
      ),
      Timeless.ui.PopoverPrimitive.Portal({ store }, [
        Timeless.ui.PopoverPrimitive.Content(
          {
            ...content_props,
            store,
            zIndex: 9999,
            class: computed(presence_state_, (state) => {
              const enter_class = "animate-in fade-in-0 zoom-in-95";
              const exit_class = "animate-out fade-out-0 zoom-out-95";
              return [
                state.enter ? enter_class : "",
                state.exit ? exit_class : "",
                !state.mounted && was_exiting_.value ? exit_class : "",
              ]
                .filter(Boolean)
                .join(" ");
            }),
          },
          content,
        ),
      ]),
    ],
  );
}
