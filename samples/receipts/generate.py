"""Генератор тестовых чеков для проверки оплат в Invoice Keeper.

Все документы — вымышленные образцы «Демо-банка» и «Демо-кассы», на каждом есть пометка
«ОБРАЗЕЦ ДЛЯ ТЕСТИРОВАНИЯ». Суммы совпадают с демо-данными (SEED_DEMO=true).

Запуск (нужны reportlab и Pillow):
    pip install reportlab pillow
    python generate.py                 # дата операции — сегодня
    python generate.py --date 2026-11-02
"""
import argparse
import datetime as dt
import os
import random
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont
from reportlab.graphics.barcode import qrencoder
from reportlab.lib.colors import HexColor
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas

OUT = Path(__file__).resolve().parent

FONT_DIRS = ["/usr/share/fonts/truetype/dejavu", "C:/Windows/Fonts", "/Library/Fonts", "/System/Library/Fonts/Supplemental"]


def find_font(names):
    for d in FONT_DIRS:
        for n in names:
            p = os.path.join(d, n)
            if os.path.exists(p):
                return p
    raise SystemExit("Не найден шрифт с кириллицей (DejaVuSans или Arial)")


REG = find_font(["DejaVuSans.ttf", "arial.ttf", "Arial.ttf"])
BOLD = find_font(["DejaVuSans-Bold.ttf", "arialbd.ttf", "Arial Bold.ttf"])
MONO = find_font(["DejaVuSansMono.ttf", "cour.ttf", "Courier New.ttf"])
pdfmetrics.registerFont(TTFont("R", REG))
pdfmetrics.registerFont(TTFont("B", BOLD))

MONTHS = ["января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"]
SAMPLE_NOTE = "ОБРАЗЕЦ ДЛЯ ТЕСТИРОВАНИЯ INVOICE KEEPER · не является платёжным документом"


def rub(v):
    s = f"{v:,.2f}".replace(",", " ").replace(".", ",")
    return s


# ---------------------------------------------------------------- PDF: квитанция банка

def bank_pdf(path, *, title, when, rows, amount_label, amount, status, status_ok=True, currency="₽", footer_rows=()):
    W, H = 105 * mm, 190 * mm
    c = canvas.Canvas(str(path), pagesize=(W, H))
    c.setTitle(title)
    c.setFillColor(HexColor("#F2F3F5"))
    c.rect(0, 0, W, H, stroke=0, fill=1)
    c.setFillColor(HexColor("#FFFFFF"))
    c.roundRect(6 * mm, 14 * mm, W - 12 * mm, H - 22 * mm, 5 * mm, stroke=0, fill=1)

    y = H - 18 * mm
    c.setFillColor(HexColor("#1F1F1F"))
    c.setFont("B", 10)
    c.drawString(11 * mm, y, "Демо-банк")
    c.setFont("R", 7)
    c.setFillColor(HexColor("#8A8A8E"))
    c.drawRightString(W - 11 * mm, y, "тестовый образец")
    y -= 11 * mm
    c.setFillColor(HexColor("#1F1F1F"))
    c.setFont("B", 13)
    c.drawString(11 * mm, y, title)
    y -= 6 * mm
    c.setFont("R", 8)
    c.setFillColor(HexColor("#8A8A8E"))
    c.drawString(11 * mm, y, when)

    y -= 13 * mm
    c.setFont("R", 8)
    c.setFillColor(HexColor("#8A8A8E"))
    c.drawString(11 * mm, y, amount_label)
    y -= 8 * mm
    c.setFont("B", 20)
    c.setFillColor(HexColor("#1F1F1F"))
    c.drawString(11 * mm, y, f"{rub(amount)} {currency}")
    y -= 8 * mm
    c.setFont("B", 8.5)
    c.setFillColor(HexColor("#1F8A4C") if status_ok else HexColor("#C0392B"))
    c.drawString(11 * mm, y, ("✓ " if status_ok else "✕ ") + status)

    y -= 8 * mm
    c.setStrokeColor(HexColor("#E6E6E9"))
    c.line(11 * mm, y, W - 11 * mm, y)
    y -= 7 * mm
    for label, value in list(rows) + list(footer_rows):
        c.setFont("R", 7.2)
        c.setFillColor(HexColor("#8A8A8E"))
        c.drawString(11 * mm, y, label)
        y -= 4.6 * mm
        c.setFont("R", 8.6)
        c.setFillColor(HexColor("#1F1F1F"))
        c.drawString(11 * mm, y, value)
        y -= 7 * mm

    c.setFont("R", 5.6)
    c.setFillColor(HexColor("#B04A00"))
    c.drawCentredString(W / 2, 8 * mm, SAMPLE_NOTE)
    c.save()


# ---------------------------------------------------------------- кассовый чек с QR ФНС

def qr_png(data, size):
    """QR-код средствами reportlab (без дополнительных библиотек), рисуется через Pillow."""
    for version in range(2, 30):  # минимальная версия, в которую помещаются данные
        q = qrencoder.QRCode(version, qrencoder.QRErrorCorrectLevel.M)
        q.addData(data)
        try:
            q.make()
            break
        except Exception:
            continue
    n = q.getModuleCount()
    border = 4
    cell = size // (n + border * 2)
    img = Image.new("RGB", ((n + border * 2) * cell,) * 2, "#FFFFFF")
    d = ImageDraw.Draw(img)
    for r in range(n):
        for c in range(n):
            if q.isDark(r, c):
                x, y = (c + border) * cell, (r + border) * cell
                d.rectangle((x, y, x + cell - 1, y + cell - 1), fill="#000000")
    return img


def cash_receipt_png(path, *, when, items, total, kind=1, shop="ООО «Демо-Медиа»", inn="7700000000", fn="7281440500123456", fd="48211", fp="2709316512"):
    t = when.strftime("%Y%m%dT%H%M")
    qr_data = f"t={t}&s={total:.2f}&fn={fn}&i={fd}&fp={fp}&n={kind}"
    W = 620
    f = ImageFont.truetype(MONO, 22)
    fb = ImageFont.truetype(MONO, 26)
    lines = [
        ("c", shop, fb), ("c", f"ИНН {inn}", f), ("c", "Демо-касса · тестовый образец", f), ("", "", f),
        ("c", "КАССОВЫЙ ЧЕК", fb), ("c", "ПРИХОД" if kind == 1 else "ВОЗВРАТ ПРИХОДА", fb), ("", "", f),
    ]
    for name, price in items:
        lines.append(("l", name, f))
        lines.append(("r", f"1 x {rub(price)} = {rub(price)}", f))
    lines += [
        ("", "", f), ("lr", ("ИТОГО", f"={rub(total)}"), fb), ("lr", ("БЕЗНАЛИЧНЫМИ", f"={rub(total)}"), f),
        ("lr", ("В т.ч. НДС 20%", f"={rub(total / 6)}"), f), ("", "", f),
        ("lr", ("Дата", when.strftime("%d.%m.%Y %H:%M")), f), ("lr", ("ФН", fn), f), ("lr", ("ФД", fd), f), ("lr", ("ФП", fp), f),
        ("lr", ("Сайт ФНС", "www.nalog.gov.ru"), f), ("", "", f),
    ]
    qr = qr_png(qr_data, 300).resize((300, 300))
    H = 60 + len(lines) * 32 + 340 + 60
    img = Image.new("RGB", (W, H), "#FFFFFF")
    d = ImageDraw.Draw(img)
    y = 40
    for align, text, font in lines:
        if align == "c":
            d.text((W / 2, y), text, font=font, fill="#111", anchor="ma")
        elif align == "l":
            d.text((40, y), text, font=font, fill="#111")
        elif align == "r":
            d.text((W - 40, y), text, font=font, fill="#111", anchor="ra")
        elif align == "lr":
            d.text((40, y), text[0], font=font, fill="#111")
            d.text((W - 40, y), text[1], font=font, fill="#111", anchor="ra")
        y += 32
    img.paste(qr, ((W - 300) // 2, y))
    y += 320
    d.text((W / 2, y), "ОБРАЗЕЦ ДЛЯ ТЕСТИРОВАНИЯ", font=ImageFont.truetype(MONO, 18), fill="#B04A00", anchor="ma")
    img.save(path)
    return qr_data


# ---------------------------------------------------------------- «фото» экрана банка

def phone_photo_jpg(path, *, when, amount, payee, op):
    W, H = 720, 1280
    img = Image.new("RGB", (W, H), "#FFFFFF")
    d = ImageDraw.Draw(img)
    R = lambda s: ImageFont.truetype(REG, s)
    B = lambda s: ImageFont.truetype(BOLD, s)
    d.text((W / 2, 70), "Демо-банк", font=B(30), fill="#222", anchor="ma")
    d.ellipse((W / 2 - 70, 170, W / 2 + 70, 310), fill="#21A038")
    d.line((W / 2 - 35, 240, W / 2 - 8, 268, W / 2 + 40, 210), fill="#FFFFFF", width=12, joint="curve")
    d.text((W / 2, 360), "Перевод выполнен", font=B(40), fill="#111", anchor="ma")
    d.text((W / 2, 440), f"{rub(amount)} ₽", font=B(64), fill="#111", anchor="ma")
    rows = [("Получатель", payee), ("Дата и время", when.strftime("%d.%m.%Y %H:%M")), ("Комиссия", "0,00 ₽"),
            ("Номер операции", op), ("Статус", "Успешно")]
    y = 580
    for k, v in rows:
        d.text((60, y), k, font=R(26), fill="#8A8A8E")
        d.text((60, y + 36), v, font=R(32), fill="#111")
        y += 110
    d.text((W / 2, H - 70), "ОБРАЗЕЦ ДЛЯ ТЕСТИРОВАНИЯ", font=R(22), fill="#B04A00", anchor="ma")
    # «снято на телефон»: фон стола, лёгкий поворот, шум, сжатие
    bg = Image.new("RGB", (W + 260, H + 260), "#9C8F80")
    rnd = random.Random(7)
    px = bg.load()
    for _ in range(60000):
        x, y2 = rnd.randrange(bg.width), rnd.randrange(bg.height)
        v = rnd.randrange(-25, 25)
        r, g, b = px[x, y2]
        px[x, y2] = (max(0, min(255, r + v)), max(0, min(255, g + v)), max(0, min(255, b + v)))
    rotated = img.rotate(-3.5, expand=True, fillcolor="#9C8F80", resample=Image.BICUBIC)
    bg.paste(rotated, ((bg.width - rotated.width) // 2, (bg.height - rotated.height) // 2))
    bg = bg.filter(ImageFilter.GaussianBlur(0.8))
    bg.save(path, quality=72)


def not_a_receipt_jpg(path):
    img = Image.new("RGB", (900, 600), "#DDE7EE")
    d = ImageDraw.Draw(img)
    for i in range(0, 900, 60):
        d.rectangle((i, 380 - (i * 7) % 180, i + 40, 600), fill="#7FA6C2")
    d.ellipse((650, 60, 780, 190), fill="#F4C46A")
    d.text((40, 40), "Котик на отдыхе", font=ImageFont.truetype(BOLD, 40), fill="#333")
    d.text((40, 100), "Фото из отпуска, 2026", font=ImageFont.truetype(REG, 26), fill="#555")
    img.save(path, quality=80)


# ---------------------------------------------------------------- набор

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--date", help="дата операции YYYY-MM-DD (по умолчанию — сегодня)")
    args = ap.parse_args()
    day = dt.date.fromisoformat(args.date) if args.date else dt.date.today()
    when = dt.datetime.combine(day, dt.time(15, 42))
    human = f"{day.day} {MONTHS[day.month - 1]} {day.year} в 15:42 (МСК)"
    old = dt.datetime.combine(day - dt.timedelta(days=75), dt.time(11, 5))

    bank_pdf(OUT / "01_перевод_399_руб_кинотеатр.pdf", title="Чек по операции", when=human,
             amount_label="Сумма перевода", amount=399, status="Выполнено",
             rows=[("Операция", "Оплата услуг"), ("Получатель", "ООО «Демо-Медиа» (онлайн-кинотеатр)"),
                   ("Счёт списания", "Карта •••• 4521"), ("Комиссия", "0,00 ₽"),
                   ("Номер документа", "1000234567"), ("Код авторизации", "482910")])
    bank_pdf(OUT / "02_СБП_299_руб_хранилище.pdf", title="Перевод по СБП", when=human,
             amount_label="Итого", amount=299, status="Успешно",
             rows=[("Получатель", "ООО «Демо-Облако»"), ("Банк получателя", "Демо-банк"),
                   ("Телефон получателя", "+7 900 000-00-00"), ("Счёт списания", "Карта •••• 4521"),
                   ("Идентификатор операции", "B6297154512340000B10050011470501")])
    cash_receipt_png(OUT / "03_кассовый_чек_QR_399_руб.png", when=when, items=[("Онлайн-кинотеатр, подписка 1 мес.", 399)], total=399)
    phone_photo_jpg(OUT / "04_фото_экрана_банка_399_руб.jpg", when=when, amount=399, payee="ООО «Демо-Медиа»", op="5117302948")
    bank_pdf(OUT / "05_неверная_сумма_350_руб.pdf", title="Чек по операции", when=human,
             amount_label="Сумма перевода", amount=350, status="Выполнено",
             rows=[("Получатель", "ООО «Демо-Медиа» (онлайн-кинотеатр)"), ("Счёт списания", "Карта •••• 4521"),
                   ("Номер документа", "1000234890")])
    bank_pdf(OUT / "06_операция_отклонена_399_руб.pdf", title="Чек по операции", when=human,
             amount_label="Сумма перевода", amount=399, status="Операция отклонена банком", status_ok=False,
             rows=[("Получатель", "ООО «Демо-Медиа» (онлайн-кинотеатр)"), ("Причина", "Недостаточно средств"),
                   ("Номер документа", "1000235001")])
    cash_receipt_png(OUT / "07_чек_возврата_QR_399_руб.png", when=when, items=[("Онлайн-кинотеатр, подписка 1 мес.", 399)], total=399, kind=2, fd="48212", fp="1190038271")
    bank_pdf(OUT / "08_оплата_VPS_15_USD.pdf", title="Card payment receipt", when=when.strftime("%Y-%m-%d 15:42 UTC+3"),
             amount_label="Total paid", amount=15, currency="USD", status="Payment completed",
             rows=[("Merchant", "Demo Cloud Hosting Ltd."), ("Description", "VPS 2 vCPU / 4 GB, 1 month"),
                   ("Card", "VISA •••• 4521"), ("Transaction ID", "TX-7731-0042-9918")])
    bank_pdf(OUT / "09_старый_чек_399_руб.pdf", title="Чек по операции",
             when=f"{old.day} {MONTHS[old.month - 1]} {old.year} в 11:05 (МСК)",
             amount_label="Сумма перевода", amount=399, status="Выполнено",
             rows=[("Получатель", "ООО «Демо-Медиа» (онлайн-кинотеатр)"), ("Номер документа", "1000101010")])
    bank_pdf(OUT / "10_аренда_85000_руб.pdf", title="Платёжное поручение", when=human,
             amount_label="Сумма платежа", amount=85000, status="Исполнено",
             rows=[("Получатель", "ИП Демо Арендодатель"), ("Назначение платежа", "Аренда офиса за месяц, без НДС"),
                   ("Номер документа", "2026-000731")])
    not_a_receipt_jpg(OUT / "11_не_чек.jpg")
    print("Готово:", OUT)


if __name__ == "__main__":
    main()
