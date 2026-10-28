# TixoTunnel (فارسی)

TixoTunnel یک هسته تونل معکوس TCP/UDP برای لینوکس است که به‌طور مستقل در Go
نوشته شده (ماژول `backhaul.local/ir_spoof`، منتشرشده اولیه توسط `ir_spoof` —
https://t.me/ir_spoof)، به‌همراه مجموعه قدیمی `TixoTunnel.sh` و موتور آن در
پوشه `core/`. این سند مربوط به **کد منبع Go** در `cmd/` و `internal/` است که
اکنون در کنار مجموعه قبلی در همین مخزن منتشر شده است.

> English: [README.md](README.md)

## این پروژه دقیقاً چیست

این یک بازسازی مستقل از رفتار تونل‌زنی شبیه Backhaul است؛ کد منبع اصلی
Backhaul نیست و هیچ ادعایی درباره هم‌خوانی پروتکل سیم (wire) با هسته پریمیوم
اصلی ندارد. هر دو سر تونل باید همین پیاده‌سازی را اجرا کنند. پیش از تکیه بر
هر ویژگی خاص، حتماً [`docs/core.md`](docs/core.md) و
[`docs/limits.md`](docs/limits.md) را بخوانید.

## ویژگی‌های تأییدشده از روی کد منبع

- **تونل معکوس TCP بین سرور و کلاینت** با نگاشت پورت (`8080=9000`، جفت‌های
  صریح `bind:target`، و بازه‌های محدود تا ۲۵۶ لیستنر) — `internal/core`.
- **ترابرها (transports)**: `tcp`، `tcpmux`، `xtcpmux`، `ws`، `wss`،
  `wsmux`، `wssmux`، `xwsmux`، `anytls`، `tun` — `internal/transport`،
  `internal/wire`.
- **فورواردینگ UDP** روی همان جریان تأییدشده، با وضعیت جداگانه برای هر همتا،
  بافر ۶۵٬۵۰۷ بایتی و انقضای idle (`accept_udp`، `peer_idle_timeout_s`) —
  `internal/core/udp.go`.
- **تأیید هویت و رمزنگاری متقابل**: دست‌دهی HMAC-SHA256 + X25519 (`BHC4`)
  کلیدهای جهت‌دار برای AES-256-GCM، AES-128-GCM یا ChaCha20-Poly1305 می‌سازد —
  `internal/wire`.
- **تونل خام TUN/IPX** بین دو سروری که خودتان کنترل‌شان را دارید، روی شش
  پروفایل خام IPv4 (ICMP، IPIP، UDP، TCP، GRE، BIP) با پاکت رمزنگاری‌شده
  اختصاصی (`BHX2`)، پنجره ضد-replay و بازسازی بسته — `internal/ipx`،
  `internal/tun`. نیاز به روت/`CAP_NET_ADMIN`، `/dev/net/tun` و raw socket
  دارد.
- **انتخاب IP مبدأ**: اتصال خروجی را به یک IP واقعی که از قبل روی سرور فعال
  است متصل می‌کند (`dialer.local_addr`)؛ این جدا از override مبدأ خام IPX
  (`ipx.spoof_src_ip` / `ipx.spoof_dst_ip`) است که فقط بین دو همتای IPX
  از‌پیش‌پیکربندی‌شده کاربرد دارد — به
  [`docs/source-ip-fa.txt`](docs/source-ip-fa.txt) و بخش امنیت زیر مراجعه
  کنید.
- **پیکربندی TOML یا JSON**، محدود به ۱ مگابایت، فیلدهای ناشناخته رد
  می‌شوند؛ `-check` پیکربندی را بدون هیچ فعالیت شبکه یا تغییر اینترفیس
  اعتبارسنجی می‌کند — `internal/config`.
- **پروتکل PROXY** روی فورواردهای TCP برای backendهایی که آن را می‌فهمند.

## سیستم‌عامل‌های پشتیبانی‌شده

- **اجرا**: فقط لینوکس (amd64, arm64). باینری روی سایر سیستم‌عامل‌ها از
  اجرای موتور تونل خودداری می‌کند؛ `-check` و `-version` در هر جایی که Go
  بتواند بسازد کار می‌کنند.
- **ساخت (build)**: هر سیستم‌عامل با Go 1.26 یا بالاتر (طبق `go.mod`)؛ این
  مخزن با Go 1.27 ساخته و تست می‌شود.

## نصب

```bash
curl -fsSL -o install.sh https://raw.githubusercontent.com/TheTixoCloud/TixoTunnel/main/install.sh
chmod +x install.sh
./install.sh            # نصب در /usr/local/bin/tixotunnel
```

نصب‌کننده معماری amd64/arm64 را تشخیص می‌دهد، فایل انتشار متناظر
`TixoTunnel-linux-<arch>.tar.gz` را دانلود می‌کند، آن را در برابر
`SHA256SUMS` بررسی می‌کند و در صورت وجود نصب قبلی، آن را به‌صورت atomic
ارتقا می‌دهد. هیچ اسکریپت دانلودشده‌ای را اجرا نمی‌کند، هیچ سرویسی را
ری‌استارت نمی‌کند و فایل‌های پیکربندی را دست نمی‌زند. پیش از اجرا با
دسترسی بالا، محتوای `install.sh` را بررسی کنید.

> این نصب‌کننده ربطی به کنسول قدیمی `TixoTunnel.sh` ندارد؛ آن کنسول همچنان
> مجموعه جدا `core/tixotunnel-core.engine` را مدیریت می‌کند و از این تغییر
> اثر نمی‌پذیرد.

## ساخت از کد منبع

```bash
git clone https://github.com/TheTixoCloud/TixoTunnel.git
cd TixoTunnel
make build          # -> bin/tixotunnel
make test           # go test ./...
make release        # -> dist/TixoTunnel-linux-{amd64,arm64}.tar.gz + SHA256SUMS
```

## پیکربندی نمونه

سرور (`server.toml`):

```toml
[listener]
bind_addr = "0.0.0.0:3080"

[ports]
mapping = ["8080=9000", "8081=9001"]

[security]
token = "یک_رمز_منحصر_به_فرد_حداقل_۳۲_بایتی"
algorithm = "aes-256-gcm"
```

کلاینت متناظر:

```toml
[dialer]
remote_addr = "203.0.113.10:3080"

[security]
token = "همان رمز بالا"
algorithm = "aes-256-gcm"
```

اعتبارسنجی پیش از اجرا:

```bash
tixotunnel -c server.toml -check
```

### TCP در برابر UDP

با تنظیم `transport.accept_udp = true` روی ترابر `tcp`/`tcpmux`/`ws...`،
بسته‌های UDP هم از همان جریان تأییدشده عبور می‌کنند؛ نیازی به لیستنر جدا
نیست.

## نکات امنیتی

- **override مبدأ خام (`ipx.spoof_src_ip`/`spoof_dst_ip`) ناشناس‌سازی نیست
  و ابزار عمومی spoof نیست.** فقط مبدأ IP خام لایه بیرونی را بین دو همتای
  IPX که هر دو از قبل پیکربندی شده‌اند تغییر می‌دهد، آن هم روی مسیر و PSK‌ای
  که خودتان کنترل دارید. نیاز به دسترسی raw socket روی سرور خودتان دارد،
  فیلترهای ضد-spoof ارائه‌دهنده را دور نمی‌زند، IP خروجی عمومی شما را تغییر
  نمی‌دهد و هیچ‌کدام از دو طرف را از دید هم مخفی نمی‌کند. فقط روی شبکه‌ای
  استفاده کنید که مالک آن هستید یا اجازه صریح تست دارید. ببینید
  [`docs/security.md`](docs/security.md) و
  [`docs/source-ip-fa.txt`](docs/source-ip-fa.txt).
- متن ساده (plaintext) رد می‌شود مگر این‌که صریحاً
  `security.allow_insecure_plaintext = true` را تنظیم کنید و رمزنگاری را هم
  غیرفعال کنید؛ این کار را روی شبکه نامطمئن انجام ندهید.
- توکن‌ها و PSKها رمز محلی هستند، نه تله‌متری. برای هر استقرار مقدار
  یکتا بسازید، فایل رمز را با مجوز `0600` نگه دارید و هرگز آن را در مخزن،
  اسکرین‌شات یا تاریخچه خط فرمان قرار ندهید.

## عیب‌یابی

- خطای `configuration invalid: ...` از `-check` — نام فیلد مشکل‌دار را
  مشخص می‌کند؛ پیش از اجرا آن را اصلاح کنید.
- تونل بالا می‌آید اما ترافیکی رد نمی‌شود — مطمئن شوید توکن/PSK دو طرف
  یکسان است، قوانین فایروال پورت‌های لیستنر/دایلر را اجازه می‌دهند، و برای
  IPX مطمئن شوید پروتکل خام (ICMP/GRE/...) توسط مسیر شبکه drop نمی‌شود.
- IPX/TUN نیاز به روت (یا `CAP_NET_ADMIN` + `CAP_NET_RAW`) و `/dev/net/tun`
  دارد.

## به‌روزرسانی

```bash
./install.sh            # اجرای دوباره برای ارتقای atomic نصب موجود
```

## دانلود نسخه‌های منتشرشده

باینری‌های ازپیش‌ساخته `linux/amd64` و `linux/arm64` به‌همراه `SHA256SUMS`
برای هر نسخه تگ‌شده در [صفحه Releases](https://github.com/TheTixoCloud/TixoTunnel/releases)
منتشر می‌شوند. پیش از اجرای باینری دانلودشده، حتماً checksum را بررسی کنید.

## کنسول مدیریتی (`TixoTunnel.sh`)

کنسول مدیریتی bash (منوهای انگلیسی، ویزارد سرور ایران / کلاینت خارج، نگاشت
پورت، مدیریت سرویس systemd) اکنون برای هر تونل جدید از همین هسته Go منتشرشده
در این مخزن استفاده می‌کند؛ تونل‌هایی که قبلاً با موتور قدیمی AETHER-X1
ساخته شده‌اند بدون تغییر قابل مدیریت باقی می‌مانند:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/TheTixoCloud/TixoTunnel/main/TixoTunnel.sh)
```

پس از نصب، کنسول را با دستور `tixotunnel` اجرا کنید. جزئیات کامل (معماری
دو-موتوره، به‌روزرسانی، سازگاری با نصب‌های قبلی) در نسخه انگلیسی این سند
آمده است.

## مجوز

کد منبع Go در `cmd/` و `internal/` با مجوز MIT منتشر شده — ببینید
[LICENSE](LICENSE). وابستگی‌های vendor شده در `vendor/` مجوز خودشان را
حفظ می‌کنند.
