import {
  fetchBookProfile,
  fetchReadingProgress,
  saveReadingProgress,
} from "@/biz/request.js";
import ComicReaderView from "@/pages/reader/comic.js";
import WebNovelReaderView from "@/pages/reader/webnovel.js";
import EpubReaderView from "@/pages/reader/epub.js";
import PdfReaderView from "@/pages/reader/pdf.js";
import TextReaderView from "@/pages/reader/text.js";

export default function ReaderPageView(props) {
  var bookId =
    (props.view && props.view.query && props.view.query.book_id) || "";

  var book_ = ref(null);
  var chapters_ = ref(null);
  var progress_ = ref(null);
  var loading_ = ref(true);
  var error_ = ref("");
  var mode_ = ref("");

  var reqs = {
    profile: new Timeless.RequestCore(
      function () {
        return fetchBookProfile(bookId);
      },
      { client: props.client },
    ),
    progress: new Timeless.RequestCore(
      function () {
        return fetchReadingProgress(bookId);
      },
      { client: props.client },
    ),
  };

  function loadData() {
    loading_.as(true);
    error_.as("");

    // Single call for book metadata + TOC + files
    var profileP = reqs.profile.run().then(function (r) {
      if (r.error) {
        error_.as(String(r.error.message || "failed to load book"));
        return;
      }
      var profile = r.data;
      book_.as(profile);

      // Populate chapters from profile TOC (for chapter-based books)
      chapters_.as(profile.toc || []);

      // Determine reader mode from profile.reader (computed by backend)
      mode_.as(profile.reader || "default");
    });

    // Separate call for reading progress (per-member)
    var progressP = reqs.progress.run().then(function (r) {
      if (!r.error && r.data) {
        progress_.as(r.data);
      }
    });

    // Set loading to false after both complete
    Promise.all([profileP, progressP]).then(function () {
      loading_.as(false);
    });
  }

  return View(
    {
      class: "h-screen bg-white dark:bg-zinc-950",
      // store: new Timeless.ui.ScrollViewCore({}),
      onMounted() {
        if (!bookId) {
          error_.as("no book_id provided");
          loading_.as(false);
          return;
        }
        loadData();
      },
    },
    [
      // Loading state
      Show({
        when: loading_,
        ok() {
          return View(
            {
              class: "flex items-center justify-center h-full",
            },
            [View({ class: "text-sm text-zinc-500" }, ["加载中..."])],
          );
        },
      }),

      // Error state
      Show({
        when: combine({ loading: loading_, error: error_ }, (t) => {
          return !t.loading && t.error;
        }),
        ok() {
          return View(
            {
              class: "flex flex-col items-center justify-center h-full gap-4",
            },
            [
              View({ class: "text-sm text-red-500" }, [error_]),
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "outline",
                    onClick: function () {
                      props.history.back();
                    },
                  }),
                },
                ["返回书架"],
              ),
            ],
          );
        },
      }),

      // No files state
      Show({
        when: combine(
          { book: book_, loading: loading_, error: error_ },
          (t) => {
            var b = t.book;
            return (
              !t.loading && !t.error && b && (!b.files || b.files.length === 0)
            );
          },
        ),
        ok() {
          return View(
            {
              class: "flex flex-col items-center justify-center h-full gap-4",
            },
            [
              View({ class: "text-sm text-zinc-500" }, [
                "该书没有可读取的文件。",
              ]),
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "outline",
                    onClick: function () {
                      props.history.back();
                    },
                  }),
                },
                ["返回书架"],
              ),
            ],
          );
        },
      }),

      // Reader mode: comic
      Show({
        when: combine({ mode: mode_, book: book_ }, function (t) {
          return t.mode === "comic" && t.book;
        }),
        ok() {
          return ComicReaderView({
            book: book_.value,
            progress: progress_.value,
            props: props,
          });
        },
      }),

      // Reader mode: webnovel
      Show({
        when: combine({ mode: mode_, book: book_ }, (t) => {
          return t.mode === "webnovel" && t.book;
        }),
        ok() {
          return WebNovelReaderView({
            book: book_.value,
            chapters: chapters_.value,
            progress: progress_.value,
            props: props,
          });
        },
      }),

      // Reader mode: epub
      Show({
        when: combine({ mode: mode_, book: book_ }, (t) => {
          return t.mode === "epub" && t.book;
        }),
        ok() {
          return EpubReaderView({
            book: book_.value,
            progress: progress_.value,
            props: props,
          });
        },
      }),

      // Reader mode: pdf
      Show({
        when: combine({ mode: mode_, book: book_ }, (t) => {
          return t.mode === "pdf" && t.book;
        }),
        ok() {
          return PdfReaderView({
            book: book_.value,
            progress: progress_.value,
            props: props,
          });
        },
      }),

      // Reader mode: text
      Show({
        when: combine({ mode: mode_, book: book_ }, (t) => {
          return t.mode === "text" && t.book;
        }),
        ok() {
          return TextReaderView({
            book: book_.value,
            progress: progress_.value,
            props: props,
          });
        },
      }),

      // Default / fallback reader
      Show({
        when: combine({ mode: mode_, book: book_ }, (t) => {
          var m = t.mode;
          return (
            m &&
            m !== "comic" &&
            m !== "webnovel" &&
            m !== "epub" &&
            m !== "pdf" &&
            m !== "text" &&
            t.book
          );
        }),
        ok() {
          var book = book_.value;
          var firstFile = book.files && book.files[0];
          var fileUrl = firstFile ? firstFile.url : "";
          return View(
            {
              class: "flex flex-col h-full",
            },
            [
              // Top bar
              View(
                {
                  class:
                    "flex items-center gap-3 px-4 py-3 border-b border-zinc-200 dark:border-zinc-800 shrink-0",
                },
                [
                  Button(
                    {
                      store: new Timeless.ui.ButtonCore({
                        variant: "ghost",
                        size: "sm",
                        onClick: function () {
                          props.history.back();
                        },
                      }),
                    },
                    [Icon({ name: "arrow-left", size: 18 })],
                  ),
                  View({ class: "text-sm font-medium truncate" }, [book.title]),
                ],
              ),
              // Content area: iframe to file
              Show({
                when: fileUrl,
                ok() {
                  return View({ class: "flex-1" }, [
                    View({
                      tag: "iframe",
                      src: fileUrl,
                      class: "w-full h-full border-0",
                    }),
                  ]);
                },
              }),
              Show({
                when: !fileUrl,
                ok() {
                  return View(
                    {
                      class:
                        "flex items-center justify-center flex-1 text-sm text-zinc-500",
                    },
                    ["不支持的文件格式"],
                  );
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}
