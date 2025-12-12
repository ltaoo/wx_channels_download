const inserted_style = `<style>
.custom-menu {
  z-index: 99999;
  background: var(--BG-CONTEXT-MENU);
  box-shadow: 0 0 6px rgb(0 0 0 / 20%);
  border-radius: 4px;
  color: var(--weui-FG-0);
  padding: 8px;
}
.custom-menu-item {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  min-width: 6em;
  transition: background .15s ease-in-out
}
.custom-menu-item:hover {
  background: var(--FG-6);
}
.custom-menu-item-arrow {
  position: absolute;
  right: 4px;
  top: 5px;
  font-size: 18px;
  line-height: 12px;
}
.wx-footer {
  position: fixed;
  right: 0;
  bottom: 18px;
  z-index: 99998;
  text-align: center;
  font-size: 14px;
  padding: 4px 48px;
}
.wx-footer-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}
.wx-sider {
  position: relative;
  position: fixed;
  right: 24px;
  top: 60%;
  z-index: 99998;
  text-align: center;
  font-size: 14px;
}
.wx-sider-bg {
  z-index: 10;
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  opacity: 0.5;
  background-color: var(--BG-0);
}
.wx-sider-tools {
  z-index: 11;
  position: relative;
  color: var(--FG-0);
}
.wx-sider-tools-btn {
  z-index: 11;
  position: relative;
  padding: 4px;
  border-radius: 4px;
  cursor: pointer;
}
.wx-sider-tools-btn:hover {
  background: var(--FG-6);
}
</style>`;

document.head.insertAdjacentHTML("beforeend", inserted_style);

var download_icon1 = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;
var download_icon2 =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABwAAAAcCAYAAAByDd+UAAAB9ElEQVR4AeyVa3LCMAwGk16scDLCyaAnc3ddy2PnzTDDrzIW8UP6VhIh+Ro+/PkHbjY8pXTZPNw5ON1SAdijWELTOcs8Jr4n9g7HKWARe6AWVT2ZhzEdbnzdih/T7XEILCIKCriO4zi3EfkrdseEWj3T9bELbGHjH0joQomzJ2ZLhQ7E2Y2FnxubQIJsn5UNiFmB/ruGX0AvxDtf+K8Cca4wInLWXE+NArUTOdl50AIIzMxsidB7EZjHnVqjpUbn2wFxEGZmZujN4boLcIGffwmTcrlm0RW1uvMOyEl2oCphQtlaHYvMWy/ijdUWv2UFknVUE9m1Gi/PgXqjCfWvUhOsQBSjugCz9faI5LO2ai3QdTg478wOYI6aLQtbxiXVvTaIKq1Qq+cZSETdaANmcwPdam+WmB/GByMDSyaKffu1ZsWn7UBAjv46P61eBpYNKwiRstVfgPr7ttAjmAK5CGLVH1pgzoTSFdVx1QicsBi7vmhZgJZhClYgCgZ70N3GOr1hcXfmYtSpQBdYtMsniQmw9fqwMswbyuq6tndAqrTCgFopcSnDU0r5rX5w1VeQtoCZegd0A2j+jZgLNgEDbc0Z01czzsfjhE43FsA4LWCD4o3uo+rQiHMYJzTk6nUTWD2YoOAb/ZThvjtOAXcVXjz8OPAXAAD//5jl7kwAAAAGSURBVAMA8H8MSLsb1AoAAAAASUVORK5CYII=";
var download_icon3 = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;
var download_icon4 = `<svg data-v-132dee25 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg>`;

/**
 * @returns {HTMLDivElement}
 */
function download_btn1() {
  const icon_download_html = download_icon1;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-6548f11a data-v-132dee25 class="click-box op-item item-gap-combine" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;">${icon_download_html}<span data-v-132dee25="" class="text">下载</span></div>`;
  return $icon.firstChild;
}
/**
 * @returns {HTMLDivElement}
 */
function download_btn2() {
  var icon_download_html = `<div class="op-icon download-icon" data-v-1fe2ed37 style="background-image: url('${download_icon2}');"></div>`;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div class=""><div data-v-6548f11a data-v-1fe2ed37 class="click-box op-item" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;">${icon_download_html}<div data-v-1fe2ed37 class="op-text">下载</div></div></div>`;
  return $icon.firstChild;
}
/**
 * @returns {HTMLDivElement}
 */
function download_btn3() {
  var icon_download_html = download_icon3;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-132dee25 class="context-menu__wrp item-gap-combine op-more-btn"><div class="context-menu__target"><div data-v-6548f11a data-v-132dee25 class="click-box op-item" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;"></div>${icon_download_html}</div></div>`;
  return $icon.firstChild;
}
/**
 * @returns {HTMLDivElement}
 */
function download_btn4() {
  var icon_download_html = download_icon4;
  var $icon = document.createElement("div");
  $icon.innerHTML = `<div data-v-ecf44def="" class="click-box__btn small" ml-key="live-menu-share"><div data-v-ecf44def="" class="text-[20px]" style="height: 1em;">${icon_download_html}</div></div>`;
  return $icon.firstChild;
}

/**
 * @param {MenuItem[]} items
 * @param {HTMLDivElement} $dropdown
 * @returns {MenuItem[]}
 */
function render_extra_menu_items(items, $dropdown) {
  if (!window.Weui) {
    return [];
  }
  const { MenuItem } = window.Weui;
  return items
    .filter((item) => {
      return item.label && item.onClick;
    })
    .map((item) => {
      return MenuItem({
        label: item.label,
        async onClick() {
          const [err, profile] = ChannelsUtil.check_profile_existing();
          if (err) return;
          var filename = __wx_build_filename(
            profile,
            null,
            __wx_channels_config__.downloadFilenameTemplate
          );
          await item.onClick({
            profile,
            filename,
          });
          $dropdown.hide();
        },
      });
    });
}
