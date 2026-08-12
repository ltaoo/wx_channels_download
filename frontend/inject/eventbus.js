/**
 * @file Generic event bus.
 *
 * Contains only the mitt instance and DOM lifecycle events. Channels-specific events
 * extend WXE on demand via pkg/scraper/wxchannels/inject/channels.events.js.
 */
var WXE = (() => {
  var eventbus = Timeless.mitt();
  var Events = {
    /** DOM fully loaded and parsed */
    DOMContentLoaded: "DOMContentLoaded",
    /** Before DOM is fully loaded */
    DOMContentBeforeUnLoaded: "DOMContentBeforeUnLoaded",
    /** All resources finished loading */
    WindowLoaded: "WindowLoaded",
    /** Page unloading completed */
    WindowUnLoaded: "WindowUnLoaded",
    /** WeChat Official Account refresh (used by wxmp scraper) */
    OfficialAccountRefresh: "OfficialAccountRefresh",
  };
  return {
    Events: Events,
    emit: eventbus.emit,
    /** Generic on / off, for use by extension modules */
    on: eventbus.on,
    off: eventbus.off,
    /** DOM fully loaded and parsed */
    onDOMContentLoaded(handler) {
      eventbus.on(Events.DOMContentLoaded, handler);
      return () => {
        eventbus.off(Events.DOMContentLoaded, handler);
      };
    },
    /** Before DOM is fully loaded */
    onDOMContentBeforeUnLoaded(handler) {
      eventbus.on(Events.DOMContentBeforeUnLoaded, handler);
      return () => {
        eventbus.off(Events.DOMContentBeforeUnLoaded, handler);
      };
    },
    /** All resources finished loading */
    onWindowLoaded(handler) {
      eventbus.on(Events.WindowLoaded, handler);
      return () => {
        eventbus.off(Events.WindowLoaded, handler);
      };
    },
    /** Page unloading completed */
    onWindowUnLoaded(handler) {
      eventbus.on(Events.WindowUnLoaded, handler);
      return () => {
        eventbus.off(Events.WindowUnLoaded, handler);
      };
    },
  };
})();

document.addEventListener("DOMContentLoaded", function () {
  WXE.emit(WXE.Events.DOMContentLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("beforeunload", function () {
  // Triggered when user is about to leave the page (DOM still exists)
  WXE.emit(WXE.Events.DOMContentBeforeUnLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("load", function () {
  WXE.emit(WXE.Events.WindowLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("unload", function () {
  // Triggered when page is about to unload (DOM is about to be destroyed)
  WXE.emit(WXE.Events.WindowUnLoaded, {
    href: window.location.href,
  });
});
