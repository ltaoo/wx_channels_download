import { app, history, client, views, storage } from "@/store/index.js";
import NotFoundPageView from "@/pages/notfound/index.js";

function ApplicationRootView() {
  const root_view$ = history.$view;
  app.onTip((msg) => {
    const { text } = msg;
    console.log("[App] tip", text);
  });
  app.onError((err) => {
    console.error("[App] error", err);
  });

  return StandardSubViews({
    view: root_view$,
    app,
    client,
    storage,
    history,
    views,
    NotFound: NotFoundPageView,
  });
}

document.addEventListener("DOMContentLoaded", function () {
  const { innerWidth, innerHeight, location } = window;
  history.$router.prepare(location);
  app.start({
    width: innerWidth,
    height: innerHeight,
  });
  Timeless.render(ApplicationRootView(), document.querySelector("#root"));
});
