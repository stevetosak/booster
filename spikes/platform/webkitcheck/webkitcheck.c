/*
 * webkitcheck: one WebKitGTK 4.1 (GTK3) window without Wails.
 *
 * The page shows a counter that goes up every 2 s and copies it into the
 * window title, which GTK and the window manager draw, not WebKit. If the
 * page's number stays one behind the title's, WebKitGTK on its own shows each
 * update one update late; if they match, the fault is in the app that embeds
 * WebKitGTK. The text box shows whether typed keys appear at once.
 *
 * Usage: webkitcheck [always|never]   (WebKit's hardware acceleration policy;
 * always is what Wails sets by default)
 */
#include <gtk/gtk.h>
#include <string.h>
#include <webkit2/webkit2.h>

static const char *page =
    "<!doctype html><html><head><meta charset='utf-8'><style>"
    "body{background:#18181b;color:#e4e4e7;font:20px sans-serif;padding:16px}"
    "#n{font:72px monospace}"
    "textarea{width:90%;height:120px;font:20px monospace;background:#27272a;color:#e4e4e7}"
    "</style></head><body>"
    "<div>Page counter (compare with the window title):</div><div id='n'>0</div>"
    "<p>Type here:</p><textarea autofocus></textarea>"
    "<script>"
    "let n = 0;"
    "setInterval(() => {"
    "  n++;"
    "  document.getElementById('n').textContent = n;"
    "  document.title = 'Title counter: ' + n;"
    "}, 2000);"
    "</script></body></html>";

static void on_title(WebKitWebView *view, GParamSpec *pspec, gpointer window) {
  const char *title = webkit_web_view_get_title(view);
  gtk_window_set_title(GTK_WINDOW(window), title && *title ? title : "webkitcheck");
}

static WebKitHardwareAccelerationPolicy parse_policy(int argc, char **argv) {
  if (argc > 1 && strcmp(argv[1], "never") == 0) {
    return WEBKIT_HARDWARE_ACCELERATION_POLICY_NEVER;
  }
  return WEBKIT_HARDWARE_ACCELERATION_POLICY_ALWAYS;
}

int main(int argc, char **argv) {
  gtk_init(&argc, &argv);

  GtkWidget *window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
  gtk_window_set_title(GTK_WINDOW(window), "webkitcheck");
  gtk_window_set_default_size(GTK_WINDOW(window), 800, 500);
  g_signal_connect(window, "destroy", G_CALLBACK(gtk_main_quit), NULL);

  WebKitWebView *view = WEBKIT_WEB_VIEW(webkit_web_view_new());
  webkit_settings_set_hardware_acceleration_policy(webkit_web_view_get_settings(view), parse_policy(argc, argv));
  g_signal_connect(view, "notify::title", G_CALLBACK(on_title), window);
  gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(view));
  webkit_web_view_load_html(view, page, NULL);

  gtk_widget_show_all(window);
  gtk_main();
  return 0;
}
