# Kendi arka planların: ASCII art, animasyon ve resimler

Arka plan (backdrop), terminalindeki yazının arkasında duran şeydir.
Limoni dört hazır sahneyle geliyor (aurora, city, starfield, synthwave). Bu
rehber kendininkini yapmakla ilgili: bir resim, bir ASCII art, hareket
eden bir ASCII art ya da kendi sahnesini çizen bir program.

Buradaki her şey iki yerde çalışır:

- **kabuğunun arkasında**, [`backdrop-shell`](../../apps/backdrop-shell/README.md)
  ile, açtığın her terminalde;
- **bir Limoni uygulamasının arkasında**, `limoni.WithBackdrop(...)` ile.

> English: [docs/backdrop-art.md](../backdrop-art.md)

---

## 1. On saniyede dene

```bash
# Örnek art dosyaları; depoyu değil de curl ile kurduysan önce indir:
mkdir -p ~/.config/limoni/art && cd ~/.config/limoni/art
for f in cat rain bird clouds lemon; do
  curl -fsSLO https://raw.githubusercontent.com/thebanri/limoni/main/apps/backdrop-shell/art/$f.txt
done

backdrop-shell -art cat.txt                               # göz kırpan bir kedi
backdrop-shell -art rain.txt                              # yağmur
backdrop-shell -image ~/Pictures/wallpaper.jpg            # bir resim
```

Çıkmak için `exit` yaz. Her yeni terminalde kalıcı olsun istersen:

```bash
backdrop-shell enable -art ~/benim-art.txt
backdrop-shell enable -image ~/Pictures/wallpaper.jpg -opacity 0.3
backdrop-shell enable -scene aurora                        # hazır sahneye geri dön
```

`backdrop-shell status` neyin ayarlı olduğunu gösterir, `backdrop-shell
disable` kapatır. Değişiklikler açık olan her terminale anında uygulanır.
Arka planı istediğin an daha sönük ya da daha belirgin yapmak için:

```bash
backdrop-shell opacity 0.3     # ya da adım adım: +0.1 / -0.1
```

---

## 2. İlk art dosyan

Art dosyası düz bir metin dosyasıdır. Metin, resmin **ta kendisidir**:

```text
  /\_/\
 ( o.o )
  > ^ <
```

Bunu `kedi.txt` olarak kaydet ve `backdrop-shell -art kedi.txt` çalıştır.
Hepsi bu kadar. Rehberin geri kalanı, onu istediğin gibi göstermek ve
hareket ettirmekle ilgili.

Baştan bilmen gereken üç kural:

1. **Boşluk saydamdır.** Art'ta boşluk olan her yerde terminalin kendi arka
   planını görürsün. Resmin şekli, karakterlerinin şeklidir.
2. **Her karakter bir sütun.** Harfler, rakamlar, noktalama, kutu çizgileri
   (`─│┌┐└┘╭╮`), bloklar (`▀▄█░▒▓`), Braille (`⣿⡇`) ve çoğu sembol bir
   sütun genişliğindedir ve çalışır. Emoji ve Çince/Japonca karakterler iki
   sütundur; satırlar kaymasın diye boşlukla değiştirilir. Tab karakteri
   sekizin bir sonraki katına atlar.
3. **Kabuğunun yazısı art'ın üstüne çizilir.** Art sadece hiçbir şey
   yazılmayan yerlerde görünür. Bir köşeye koy ya da üstünden okunabilecek
   kadar sade tut.

---

## 3. Ayarlar: `@` ile başlayan satırlar

İlk sütunundan başlayarak aşağıdaki ayarlardan biriyle başlayan satır,
resmin parçası olmaz; art'ın nasıl gösterileceğini değiştirir. Diğer her
satır art'tır, yani resmindeki `@@@@` ya da `@home` olduğu gibi çizilir.

| Ayar | Ne yapar | Örnek |
| :--- | :--- | :--- |
| `@align` | art'ın yeri: `center`, `top`, `bottom`, `left`, `right`, `top-left`, `top-right`, `bottom-left`, `bottom-right` | `@align bottom-right` |
| `@offset` | oradan sütun ve satır olarak kaydırır (eksi: sola/yukarı) | `@offset -3 -1` |
| `@color` | bir renk, ya da geçiş için birkaç renk | `@color #ffd23f` |
| `@gradient` | geçişin yönü: `vertical`, `horizontal`, `diagonal` | `@gradient horizontal` |
| `@background` | tüm ekranın arkasına bir renk | `@background #0b1020` |
| `@frame` | animasyonun bir sonraki karesini başlatır | `@frame` |
| `@fps` | saniyedeki kare sayısı (varsayılan 4) | `@fps 2` |
| `@scroll` | art'ı saniyede şu kadar sütun ve satır kaydırır | `@scroll 5 0` |
| `@tile` | art'ı ekranı dolduracak şekilde tekrarlar; iki sayı aralık bırakır | `@tile 10 3` |

Ayarlar her yerde olabilir ama en üstte okunması en kolay. Bir hata satır
numarasıyla bildirilir (`line 2: @fps wants a number from 0 to 60`) ve
backdrop-shell seni kabuksuz bırakmak yerine aurora'ya geri döner.

### Yerleştir

```text
@align bottom-right
@offset -3 -1
      __
    .'  '.
   (  ()  )
    '.__.'
```

`bottom-right` art'ı köşeye koyar. `@offset -3 -1` onu sağ kenardan üç
sütun içeri, alttan bir satır yukarı çeker; böylece pencerenin kenarına
değmez.

### Renklendir

```text
@color #fff6a8 #ffd23f #d99a00
@gradient vertical
```

Tek renk her karakteri o renge boyar. İki ya da daha fazla renk bir geçiş
(gradyan) oluşturur: burada yukarıda soluk sarıdan, limon sarısından
geçerek aşağıda koyu kehribara. `horizontal` soldan sağa, `diagonal` sol
üst köşeden başlayarak gider.

`@color` yoksa art soluk gri-mavi bir renkle çizilir. Bu bilerek kabuk
yazından daha sönük seçildi, böylece resim hiçbir zaman komut çıktısıyla
karışmaz.

Ardından `-opacity` (varsayılan `0.45`) hepsini terminalinin kendi arka plan
rengine doğru soldurur: `0.2` fısıltı gibidir, `1` tam güçtür.

---

## 4. Animasyon, birinci kısım: kareler

Bir çevir-kitap (flipbook). Her resmi çiz, aralarına `@frame` koy ve
saniyede kaç tane gösterileceğini ayarla:

```text
@fps 2
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( -.- )
```

Bu kedi göz kırpar, ama ömrünün yarısını gözleri kapalı geçirir; canlı değil,
uykulu görünür. **Zamanlama kareleri tekrarlayarak yapılır.** `@fps 2`'de
her kare yarım saniye sürer. Kapalı göz en sonda olmak üzere dört kare
koyarsan kedi bir buçuk saniye bakar, yarım saniye göz kırpar:

```text
@fps 2
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( -.- )
```

`apps/backdrop-shell/art/cat.txt` tam olarak bunu yapıyor ve aynı ritimde
kuyruğunu da sallıyor.

İpuçları:

- Tüm kareleri aynı boyda tut. Kareler sol üst köşelerinden hizalanır ve
  art en büyük kareye göre yerleştirilir. Bir sütun daha geniş olan kare
  hiçbir şeyi kaydırmaz, ama bir karede resmi bir sütun sağa çizersen resim
  zıplar.
- Kareler arasında olabildiğince az şey değiştir. Terminale sadece değişen
  karakterler gönderilir; ucuz kalmasının sebebi bu.
- Arka plan için `@fps 1` ile `@fps 4` arası uygundur. Daha hızlısı gözünü
  işinden çeker.

---

## 5. Animasyon, ikinci kısım: hareket

`@scroll` art'ın tamamını hareket ettirir: saniyede şu kadar sütun sağa
(eksi: sola), şu kadar satır aşağı (eksi: yukarı). Ekranın bir tarafından
çıkan, diğer tarafından geri gelir.

```text
@scroll 5 0
@align top-left
@offset 0 3
  _/v\_
```

Bu kuş ekranı saniyede beş sütun hızla geçer. İki kare verirsen uçarken
kanat çırpar; kareler ve hareket birlikte kullanılabilir:

```text
@fps 4
@scroll 5 0
@align top-left
@offset 0 3
 __   __
   \v/
@frame
  _/v\_
```

(`apps/backdrop-shell/art/bird.txt`)

### Ekranı dolduran desenler: `@tile`

`@tile` art'ı duvar kağıdı gibi bütün ekrana tekrarlar. `@scroll` ile
birlikte bütün desen hareket eder; hava durumu böyle yapılır:

```text
@color #4a6fa5 #2c4466
@tile
@scroll 0 14
   |           '         |              .          |       '        |
         .         |              '           |          .
 '            |        .     |         .              '        |
       |            '                |        '    |                 .
            '    .        |      '                     .      |
   .       |          '        .          |     '                |    '
```

Bu `rain.txt`: dümdüz aşağı, saniyede on dört satır. Döşenen bir desenin
doğal görünmesini iki şey sağlar:

- **Deseni geniş ve düzensiz yap.** Göz tekrarı çabuk yakalar. 60 sütun ya
  da daha geniş, damlaları düzensiz aralıklarla yerleştirilmiş bir desen
  tekrarı gizler.
- **`@tile x y` ile aralık bırak.** `@tile 10 3` kopyaların arasına on
  sütun ve üç satır koyar. Birbirine değmemesi gereken bulutlar için tam
  uygun:

```text
@color #5b6a99 #34406a
@tile 10 3
@scroll 1.5 0
      .--.
   .-(    ).
  (___.__)__)
```

Arka plan için yavaş olan daha iyidir: bulutlar `1.5`, yağmur `10`–`15`,
kar hafif bir yan kaymayla `@scroll 1 3`.

---

## 6. Başka araçlardan art

Bir programın renkli yazdığı her şey kaydedilip art olarak kullanılabilir,
çünkü backdrop-shell dosyadaki renk kodlarını okur (ANSI SGR: 16 renk, 256
renk ve truecolor; hem yazı hem arka plan için). Metindeki renk `@color`'a
baskın gelir.

```bash
# Bir resmi renkli blok art'a çevir (chafa: https://hpjansson.org/chafa/)
chafa --size 100x30 --format symbols foto.jpg > foto.txt

# Bir resmi renkli harflere çevir
jp2a --colors --width=100 foto.jpg > foto.txt

# Gökkuşağı renginde büyük yazı
figlet -f slant "merhaba" | lolcat -f > merhaba.txt
toilet -f future --gay "merhaba" > merhaba.txt
```

Sonra herhangi bir editörle en üste ayarları ekle, örneğin
`@align bottom-right` ya da yavaşça kaysın diye `@scroll`. Sonra diğer art'lar
gibi kullan.

Araç çıktısı hakkında iki şey:

- `chafa`'nın blok art'ı **boşlukları** arka plan rengiyle boyar. O boşluklar
  çizilir, çünkü resmin parçasıdırlar. Sadece renksiz boşluklar saydamdır.
- `--size`/`--width`'i terminalinden küçük seç. Ekrandan büyük art
  kenarlardan kesilir.

---

## 7. Kendi sahnen: bir program

Art bir kez, tek bir boyutta çizilir. Ekran hangi boyutta olursa olsun onu
dolduran ve hazır sahneler gibi hareket eden bir sahne ise bir programdır —
herhangi bir dilde — ve `-scene-cmd` ile verilir:

```bash
backdrop-shell -scene-cmd ~/.config/limoni/scenes/fireflies.py
backdrop-shell enable -scene-cmd ~/.config/limoni/scenes/fireflies.py   # her terminalde
```

`~/.config/limoni/scenes` içindeki bir program aynı zamanda adıyla seçilen
bir sahnedir ve hazır sahnelerle birlikte listelenir: oradaki `fireflies.py`
`-scene fireflies` olur; kendi adında bir program taşıyan `cosmos/cosmos`
klasörü de `-scene cosmos`.

Eksiksiz bir örnek:
[`apps/backdrop-shell/scenes/fireflies.py`](../../apps/backdrop-shell/scenes/fireflies.py).
Kopyala ve çizdiğini değiştir. Olabilecek en küçük sahne, ekranın ortasında
sağa sola sallanan bir nokta:

```python
#!/usr/bin/env python3
import math, os, sys, time

cols, rows = int(os.environ["LIMONI_COLS"]), int(os.environ["LIMONI_ROWS"])
t = 0.0
while True:
    x = int((math.sin(t) + 1) / 2 * (cols - 1))
    # Bir kare: temizle, ortadaki satıra git, sarı bir nokta çiz.
    sys.stdout.write(f"\x1b[2J\x1b[{rows // 2 + 1};{x + 1}H\x1b[38;2;255;210;60m•")
    sys.stdout.flush()
    t += 0.1
    time.sleep(0.1)
```

Kaydet, `chmod +x` ile çalıştırılabilir yap ve yolunu `-scene-cmd`'ye ver.
Kurallar:

- **Çalıştırılabilir bir dosya olmalı.** Betik `#!` ile başlar ve
  `chmod +x` ile çalıştırılabilir yapılır. Argüman vermek için asıl
  programı o argümanlarla çağıran iki satırlık bir betik yaz.
- **Ekran boyutunda bir terminalde çalışır.** Boyut `LIMONI_COLS` ve
  `LIMONI_ROWS`'ta; `stty size`, `tput cols` ve Python'un
  `shutil.get_terminal_size()`'ı da verir.
- **Her terminalde çizdiği gibi çizer:** renkler (yazı için
  `\x1b[38;2;R;G;Bm`, arka plan için `\x1b[48;2;R;G;Bm`), imleci taşımak
  (`\x1b[SATIR;SÜTUNH`, 1'den sayarak), temizlemek (`\x1b[2J`).
- **Her kare temizleyerek ya da başa dönerek başlar** — `\x1b[2J` ya da
  `\x1b[H`. Bir kare, bir sonraki başladığında gösterilir; böylece yarım
  çizilmiş bir kare hiç görünmez. Program kareler arasında sustuğunda da
  çizdiği gösterilir; tek bir resim çizip uyuyan bir program da çalışır.
- **Her kareden sonra flush et** (`flush()`, `fflush(stdout)`), yoksa kare
  programın kendi tamponunda bekler.
- **Arka plan rengi olmayan hücre terminalin kendi arka planını gösterir**,
  art'taki boşluk gibi; rengi olan hücre sahnenin parçasıdır. `-opacity`
  ikisini de her sahnedeki gibi soldurur.

Gerisini backdrop-shell halleder. Pencerenin boyutu değişince program yeni
boyutla yeniden başlatılır. Pencere odakta değilken ya da tam ekran bir
program arka planı kapatırken program durdurulur ve hiçbir şeye mal olmaz;
arka plan yeniden görününce kaldığı yerden devam eder. Başka bir arka plan
seçildiğinde ya da terminal kapandığında, başlattığı her şeyle birlikte
sonlandırılır. `-fps` ve `-still` geçerli değildir: hızını program belirler.

Hata çıktısı atılır, çünkü ekran kabuğuna aittir. Arka plan boş kalıyorsa
programı bir terminalde tek başına çalıştır —
`~/.config/limoni/scenes/fireflies.py` — ve ne dediğini oku.

Terminale sadece bir kareden diğerine değişen gönderilir; yani her karede
her şeyi yeniden çizmek sorun değil. Maliyetli olan, yumuşakça kayan
renklerdir: her karede her hücreyi değiştirirler. Renkleri `fireflies.py`
gibi birkaç basamağa yuvarla; bir hücre sadece bir basamağı geçince yeniden
gönderilir.

---

## 8. Duvar kağıdı olarak resim

```bash
backdrop-shell -image ~/Pictures/wallpaper.jpg -opacity 0.3
```

PNG, JPEG ve GIF (ilk karesi) çalışır. Resim ekranı kaplar: hiçbir kenar
boş kalmayana kadar ölçeklenir, taşan kısım iki yandan eşit kırpılır. Hücre
başına iki piksel olacak şekilde yarım bloklarla çizilir. Fotoğraf gibi
değil, piksel sanatı gibi görünür; bir terminal hücresi iki renk
taşıyabilir ve her terminalde var olan çözünürlük bu kadar.

En iyi sonucu koyu ve yumuşak resimler verir: yıldızlı bir gökyüzü, geceleri
bulanık bir şehir, bir renk geçişi. Kalabalık, parlak bir fotoğraf önündeki
yazıyla savaşır; `-opacity`'yi o savaş bitene kadar düşür.

Resim hareketsiz olduğu için çalışırken hiçbir maliyeti yoktur: terminal
açılınca bir kez, pencere boyutu değişince bir kez daha çizilir.

---

## 9. Maliyeti

Terminale sadece değişen karakterler gönderilir, yani bir arka plan ne kadar
hareket ediyorsa o kadar maliyetlidir. kitty'de, fish'in arkasında, 120×40,
her biri yirmi saniye, tüm iş parçacıklarının CPU süresiyle ölçüldü:

| Arka plan | backdrop-shell | kitty |
| :--- | ---: | ---: |
| resim (`-image`) | %0.00 | %0.01 |
| aurora, `-still` | %0.01 | %1.82 |
| `cat.txt` (2 fps) | %0.03 | %0.29 |
| `rain.txt` (saniyede 14 satır) | %0.22 | %2.30 |
| starfield | %0.33 | %1.25 |
| aurora | %0.54 | %1.26 |
| synthwave | %0.77 | %1.45 |

(Tek bir çekirdeğin yüzdesi. kitty'nin payı o sırada başka ne yaptığına ve
pencere odağına göre değişiyor; duran aurora ile hareket eden aurora ona
aşağı yukarı aynı maliyette.)

Pencere odakta değilken ve btop gibi tam ekran bir program arka planın
tamamını kapatırken her şey durur. Herhangi bir arka planı daha ucuz yapmak
için: daha az kare (`@fps`, hazır sahneler için `-fps 10`), daha az değişen
karakter ya da `-still`.

---

## 10. Bir şey yanlış görünüyorsa

| Gördüğün | Sebebi ve ne yapmalı |
| :--- | :--- |
| Art donmuş | Ayarlarda `still = true` var (`backdrop-shell status` bunu söyler). `backdrop-shell enable -still=false` hareket ettirir; `enable -art …` ile yeni bir arka plan seçmek de sıfırlar; `backdrop-shell reset` her şeyi baştan alır. |
| Kopyalayınca art da geliyor | Bu terminalin kendi seçimi (Shift+sürükle ya da `select = false`). Düz sürükleme backdrop-shell'de seçer ve sadece kabuğun yazdığını kopyalar. |
| Bir ayar yazı olarak çiziliyor | İlk sütundan başlamalı; girintili bir `@fps` resmin parçasıdır. Sadece 3. bölümdeki tablodaki isimler ayardır. |
| Satırlar yamuk | Tab'lar (sekizin katlarına atlar) ya da iki sütunluk karakterler (boşlukla değiştirilir). Boşluk ve tek sütunluk karakterler kullan. |
| Hiçbir şey yok | 16 renkli bir terminal, `LIMONI_BACKDROP=off`, ya da zaten backdrop-shell'in içindesin (`backdrop-shell status` bunu söyler). |
| Programın sahnesi boş kalıyor | Hatalarını görmek için programı tek başına çalıştır; kabuğun arkasındayken hata çıktısı atılır. `chmod +x` yapıldığını, `#!` ile başladığını ve her kareden sonra flush ettiğini kontrol et. |
| Art bir programın arkasında kayboluyor | Kendi arka planını boyayan programlar (btop, bir vim renk teması) onu kapatır. Bu bilerek böyle. |

---

## 11. Go programcıları için

Aynı art ve resimler her Limoni uygulamasında çalışır:

```go
art, err := backdrop.LoadArt("kedi.txt")   // ya da backdrop.ParseArt(reader)
if err != nil { ... }
limoni.Run(app, limoni.WithBackdrop(art))

img, err := backdrop.LoadImage("wallpaper.jpg")
limoni.Run(app, limoni.WithBackdrop(backdrop.Fade(img, bgRengi, 0.3)))
```

### Kendi sahneni yazmak

Sahne, iki metodu olan herhangi bir şeydir (`terminal.Backdrop`):

```go
type Backdrop interface {
    Render(dst *buffer.Buffer, t time.Duration) // t anı için her hücreyi boya
    Interval() time.Duration                    // ne sıklıkla değişir; 0 = hareketsiz
}
```

İşte eksiksiz bir örnek: ekranda aşağı doğru süzülen bir ışık bandı.

```go
type sweep struct{}

func (sweep) Interval() time.Duration { return time.Second / 15 }

func (sweep) Render(dst *buffer.Buffer, t time.Duration) {
    w, h := int(dst.Area.Width), int(dst.Area.Height)
    // Bandın yeri: her dört saniyede ekranı bir kez aşağı geçer.
    band := math.Mod(t.Seconds()/4, 1) * float64(h+8) - 4
    for y := 0; y < h; y++ {
        d := math.Abs(float64(y) - band)
        // Sekiz basamağa yuvarlanmış: bir satır her karede değil, sadece
        // bant bir basamağı geçince değişir.
        k := math.Floor(math.Max(0, 1-d/4)*8) / 8
        bg := cell.NewColorRGB(uint8(8+k*30), uint8(10+k*40), uint8(24+k*60))
        for x := 0; x < w; x++ {
            dst.Content[y*w+x] = cell.Cell{Content: ' ', Style: cell.Style{Bg: bg}}
        }
    }
}
```

Bir sahneyi ucuz tutan kurallar ve sebepleri:

1. **Her hücreyi boya.** `dst` hâlâ bir önceki kareyi tutuyor.
2. **`t`'nin bir fonksiyonu ol.** Kendi tuttuğun bir sayaçtan değil, sana
   verilen zamandan çiz. Aynı an iki kez çizilebilir (bir kez uygulama için,
   bir kez sadece arka plan için) ve ikisi aynı görünmelidir.
3. **Kayan değerleri yuvarla.** Yumuşak bir eğriden hesaplanan renk her
   karede farklıdır, yani her hücre her karede yeniden gönderilir. Onu bir
   avuç basamağa yuvarla; hücre sadece bir basamağı geçince değişsin.
4. **Büyük alanlarda boşluğun arkasına arka plan rengi koy.** Kodlayıcı
   aynı renkteki bir diziyi tek renk ve bir tekrar komutu olarak gönderir.
5. **Hareket etmeyen şeyi her boyut için bir kez hesapla** ve her karede
   kopyala.
6. **`Render` içinde bellek ayırma.** Tamponları sahnede tut; boyut değişince
   oluştur.

Bir sahnenin maliyetini görmek için onu bir tampona çiz,
`buffer.DiffWithOptions` ile bir önceki kareyle karşılaştır ve saniyedeki
baytları say. `backdrop/backdrop_test.go`'daki `traffic` tam olarak bunu
yapıyor ve `TestTheGuidesExampleScene` onu yukarıdaki sweep üzerinde
çalıştırıyor: saniyede 2.7 KB. Tam yeniden çizimle de karşılaştır ama oranı
dikkatle oku: düz bir sahneyi baştan çizmek zaten neredeyse bedavadır (satır
başına bir renk ve bir tekrar), bu yüzden ucuz bir sahne bile bunun büyük bir
payı gibi görünebilir.
