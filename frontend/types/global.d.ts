export {};

declare global {
  type LogMsg = {
    msg: string;
    prefix?: string;
    ignore_prefix?: 1;
    replace?: 1;
    end?: 1;
  };

  interface Window {
    WXU: {
      log: (payload: LogMsg) => void;
    };
    __CONFIG__: {
      apiServerAddr: string;
    };
  }

  const axios: import("axios").AxiosStatic;

  type RouteConfigure = typeof import("../src/store/index.js").routes_configure;
  type PageKey = import("@timeless/kit").BuildRoutesPageKeys<RouteConfigure>;
  type RouteConfig<T> = import("@timeless/kit").RouteConfig<T>;
  type RouteViewCore = import("@timeless/kit").RouteViewCore;
  type HistoryCore = import("@timeless/kit").HistoryCore<
    PageKey,
    RouteConfig<PageKey>
  >;
  type ApplicationModel = import("@timeless/kit").ApplicationModel<any>;
  type HttpClient = import("@timeless/kit").HttpClientCore;
  type StorageCore = import("@timeless/kit").StorageCore<any>;

  type ViewComponentProps = {
    view: RouteViewCore;
    views: Record<PageKey, any>;
    history: HistoryCore;
    app: ApplicationModel;
    client: HttpClient;
    storage: StorageCore;
  };
}
