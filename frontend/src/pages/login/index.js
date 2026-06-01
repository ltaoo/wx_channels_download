import { user$ } from "@/store/index.js";
import { memberLogin } from "@/biz/request.js";

export default function LoginPage(props) {
  async function handleLogin() {
    const token = ui.input_token.value.trim();
    if (!token) {
      alert("Please enter your member token");
      return;
    }

    ui.btn_login.loading = true;
    try {
      const r = await memberLogin(token);
      if (r.error) {
        alert("Invalid token: " + (r.error?.msg || r.error));
        ui.btn_login.loading = false;
        return;
      }
      const member = r.data;
      user$.login({
        id: member.id,
        username: member.name || member.remark || "Member",
        email: member.email || "",
        token: token,
      });
      const redirect = props.view?.query?.redirect;
      const redirectQuery = (() => {
        const raw = props.view?.query?.redirect_query;
        if (!raw) return {};
        try {
          return JSON.parse(decodeURIComponent(String(raw)));
        } catch {
          return {};
        }
      })();
      if (redirect) {
        props.history.replace(redirect, redirectQuery);
        return;
      }
      props.history.replace("root.home_layout.books");
    } catch (err) {
      alert("Login failed: " + (err?.message || err));
      ui.btn_login.loading = false;
    }
  }

  const ui = {
    input_token: new Timeless.ui.InputCore({
      defaultValue: "",
      placeholder: "Paste your member token",
    }),
    btn_login: new Timeless.ui.ButtonCore({
      loading: false,
      onClick: handleLogin,
    }),
  };

  return View(
    {
      class: classNames([
        "flex min-h-screen flex-col items-center justify-center bg-gray-100 py-12 sm:px-6 lg:px-8",
        "dark:bg-zinc-900",
      ]),
    },
    [
      View({ class: classNames(["sm:mx-auto sm:w-full sm:max-w-md"]) }, [
        View(
          {
            class: classNames([
              "mx-auto text-center text-3xl font-bold tracking-tight text-gray-900",
              "dark:text-white",
            ]),
          },
          ["Ink Reader"],
        ),
        View(
          {
            class: classNames([
              "mt-2 text-center text-sm text-gray-600",
              "dark:text-zinc-400",
            ]),
          },
          ["Sign in with your member token"],
        ),
      ]),

      View({ class: classNames(["mt-8 sm:mx-auto sm:w-full sm:max-w-md"]) }, [
        View(
          {
            class: classNames([
              "py-8 px-4 shadow sm:rounded-lg sm:px-10 space-y-6",
            ]),
          },
          [
            // Token Input
            View({ class: classNames(["space-y-1"]) }, [
              Label(
                {
                  class: classNames([
                    "block text-sm font-medium text-gray-700",
                    "dark:text-zinc-300",
                  ]),
                },
                ["Member Token"],
              ),
              View({ class: "mt-1" }, [
                Textarea({
                  store: ui.input_token,
                  rows: 3,
                  class: classNames([
                    "block w-full appearance-none rounded-md border border-gray-300 px-3 py-2 placeholder-gray-400 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-indigo-500 sm:text-sm",
                    "dark:bg-zinc-900 dark:border-zinc-700 dark:text-white dark:placeholder-zinc-500",
                  ]),
                }),
              ]),
            ]),

            // Login Button
            View({}, [
              Button(
                {
                  store: ui.btn_login,
                  class: classNames([
                    "flex w-full justify-center rounded-md border border-transparent bg-indigo-600 py-2 px-4 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2",
                    "dark:bg-indigo-500 dark:hover:bg-indigo-600",
                  ]),
                },
                [ui.btn_login.loading ? "Verifying..." : "Sign in"],
              ),
            ]),

            // Hint
            View(
              {
                class: classNames([
                  "mt-6 text-center text-xs text-gray-500",
                  "dark:text-zinc-500",
                ]),
              },
              [
                "Get your token by running: ",
                View(
                  {
                    as: "code",
                    class: classNames([
                      "font-mono text-gray-700 bg-gray-200 px-1 rounded",
                      "dark:text-zinc-300 dark:bg-zinc-800",
                    ]),
                  },
                  ["go run cmd/cli admin"],
                ),
              ],
            ),
          ],
        ),
      ]),
    ],
  );
}
