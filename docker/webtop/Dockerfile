FROM lscr.io/linuxserver/webtop:ubuntu-xfce

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=zh_CN.UTF-8 \
    LC_ALL=zh_CN.UTF-8 \
    LIBGL_ALWAYS_SOFTWARE=1 \
    WX_VIDEO_HOME=/config/wx_video_download \
    WX_CHANNELS_DOWNLOAD_CONFIG_FILEPATH=/config/wx_video_download/config.yaml \
    WECHAT_BIN=/usr/bin/wechat

COPY SunnyRoot.cer /usr/local/share/ca-certificates/SunnyRoot.crt
COPY wx_video_download /opt/wx_video_download/wx_video_download
COPY config.yaml /opt/wx_video_download/config.yaml
COPY global.js /opt/wx_video_download/global.js
COPY WeChatLinux_arm64.deb /tmp/WeChatLinux_arm64.deb

RUN set -eux; \
    sed -i '/^api:/,/^[^[:space:]]/s/^\(  hostname:[[:space:]]*\).*/\1"0.0.0.0"/' /opt/wx_video_download/config.yaml; \
    sed -i '/^proxy:/,/^[^[:space:]]/s/^\(  hostname:[[:space:]]*\).*/\1"0.0.0.0"/' /opt/wx_video_download/config.yaml; \
    sed -i '/^proxy:/,/^[^[:space:]]/s/^\(  tun:[[:space:]]*\).*/\1true/' /opt/wx_video_download/config.yaml

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        locales \
        procps \
        psmisc \
        iproute2 \
        iptables \
        nftables \
        sudo \
        libcap2-bin \
        xdotool \
        xclip \
        libnss3-tools \
        fonts-wqy-zenhei \
        fonts-wqy-microhei \
        fonts-noto-cjk \
        fonts-noto-color-emoji \
        libnss3 \
        libgbm1 \
        libasound2t64 \
        libxss1 \
        libatomic1 \
        libxdamage1 \
        libxkbcommon-x11-0 \
        libxcb-icccm4 \
        libxcb-image0 \
        libxcb-keysyms1 \
        libxcb-render-util0 \
        libxcb-xkb1 \
        libxcb-cursor0 \
        libgtk-3-0t64 \
        libatk1.0-0t64 \
        libatk-bridge2.0-0t64 \
        libatspi2.0-0t64 \
        libcups2t64 \
        libxcomposite1 \
        libxrandr2 \
        libxfixes3 \
        libxtst6 \
        libxshmfence1 \
        libdrm2; \
    apt-get install -y --no-install-recommends /tmp/WeChatLinux_arm64.deb; \
    sed -i 's/# *zh_CN.UTF-8 UTF-8/zh_CN.UTF-8 UTF-8/' /etc/locale.gen; \
    locale-gen; \
    update-ca-certificates; \
    mkdir -p /etc/pki/nssdb; \
    if [ ! -f /etc/pki/nssdb/cert9.db ]; then certutil -d sql:/etc/pki/nssdb -N --empty-password; fi; \
    certutil -d sql:/etc/pki/nssdb -D -n SunnyRoot >/dev/null 2>&1 || true; \
    certutil -d sql:/etc/pki/nssdb -A -n SunnyRoot -t "CT,C,C" -i /usr/local/share/ca-certificates/SunnyRoot.crt; \
    chmod +x /opt/wx_video_download/wx_video_download; \
    setcap cap_net_admin,cap_net_raw+ep /opt/wx_video_download/wx_video_download || true; \
    rm -f /tmp/WeChatLinux_arm64.deb; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*

COPY rootfs/ /
RUN chmod +x /usr/local/bin/wx-* /custom-cont-init.d/*

EXPOSE 3000 3001 2022 2023
