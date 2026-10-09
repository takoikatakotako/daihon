# movie.yml リファレンス

ファイル内のパスは、そのファイルからの相対パスで書きます。

- [全体](#全体)
- [meta](#meta)
- [imports](#imports)
- [video](#video)
- [export](#export)
- [fonts](#fonts)
- [audio](#audio)
- [subtitle](#subtitle)
- [scenes](#scenes)
- [セリフ](#セリフ)
- [演出命令](#演出命令)

## 全体

```yaml
version: 1

meta:
  title: "daihon の紹介"
  output: out/introduction.mp4
  credit: |
    VOICEVOX:春日部つむぎ
    VOICEVOX:ずんだもん

imports:
  - characters/cast.yml

video:
  width: 1920
  height: 1080
  fps: 30
  background: { src: assets/bg/street.jpg, dim: 0.15, blur: 3 }

export:
  height: 720
  crf: 28
  audio_bitrate: 64k

audio:
  pause_between_lines_ms: 350
  bgm:
    src: assets/bgm/bgm.mp3
    volume: 0.10
    fade_out_ms: 3000

subtitle:
  mode: bubble
  size: 48
  max_chars_per_line: 22

scenes:
  - id: intro
    script:
      - kouhai: "先輩、解説動画を作ってって頼まれたんですけど……"
        expr: shock
      - show: assets/images/editing.png
        as: editing
        enter: fade
      - senpai: "それなら daihon を使ってみて。"
        read: "それなら、ダイホンを使ってみて。"
      - hide: editing
```

## meta

| キー | 既定値 | 説明 |
|---|---|---|
| `title` | | 動画のタイトル |
| `output` | `out/movie.mp4` | 書き出し先 |
| `credit` | | 動画の最後に 3 秒間表示する文字列。複数行可 |

## imports

キャラクター定義などを別ファイルから読み込みます。読み込んだファイルの中のパスは、
そのファイルからの相対で解決されます。

```yaml
imports:
  - characters/cast.yml
```

## video

| キー | 既定値 | 説明 |
|---|---|---|
| `width` | `1920` | レイアウトの幅 |
| `height` | `1080` | レイアウトの高さ。字幕サイズ・オフセット・ぼかし半径はこの高さが基準 |
| `fps` | `30` | フレームレート |
| `background` | | 全シーン共通の背景 |

### background

文字列でパスだけ書くか、加工付きで書きます。PNG / JPEG / GIF / WebP が使えます。

```yaml
background: assets/bg/slide.png
background: { src: assets/bg/photo.jpg, dim: 0.35, blur: 6 }
```

| キー | 説明 |
|---|---|
| `src` | 画像のパス |
| `dim` | 0〜1。黒に寄せる割合 |
| `blur` | ぼかし半径（`video.height` 基準の px） |

写真を背景にすると立ち絵と吹き出しが埋もれやすいので、少し暗くしてぼかすと見やすくなります。
シーンごとの背景（`scenes[].background`）でも同じ書き方ができます。

## export

書き出すファイルの大きさだけを変えます。省略すると 1080p・高画質で書き出します。

| キー | 既定値 | 説明 |
|---|---|---|
| `height` | `video.height` | 書き出す高さ。幅は比率を保って決まる |
| `crf` | `18` | 画質（0〜51）。大きいほどファイルが小さく粗くなる |
| `audio_bitrate` | `192k` | 音声のビットレート。声だけなら `64k` で十分 |

```yaml
export:
  height: 720
  crf: 28
  audio_bitrate: 64k
```

レイアウトは `video` のサイズで書いたまま、全体が縮みます。`video.height` を直接 720 に
すると、字幕の大きさなど `video.height` 基準の値まで相対的に変わってしまうので、
小さく書き出したいときは `export` を使ってください。

目安（62 秒のサンプル）:

| 設定 | サイズ |
|---|---|
| 既定（1080p・CRF 18・192k） | 3.7MB |
| 720p・CRF 28・64k | 1.3MB |

## fonts

字幕のフォントを名前を付けて登録します。`subtitle.font` で名前を指定します。

```yaml
fonts:
  default: assets/fonts/NotoSansJP-Bold.ttf
```

書かなかった場合はシステムのフォントで描画し、`daihon check` が警告します。
環境によって見た目が変わらないよう、フォントはファイルで指定するのがおすすめです。

## audio

| キー | 既定値 | 説明 |
|---|---|---|
| `pause_between_lines_ms` | `400` | セリフとセリフの間の無音 |
| `bgm.src` | | BGM のパス |
| `bgm.volume` | | 音量（0〜1）。声の邪魔をしないよう 0.1 前後がおすすめ |
| `bgm.fade_out_ms` | | 動画の最後にかけてフェードアウトする長さ |

BGM は動画より短ければループします。

## subtitle

| キー | 既定値 | 説明 |
|---|---|---|
| `mode` | `bubble` | `bubble`（話者の側に吹き出し）/ `telop`（画面下にテロップ）/ `none` |
| `font` | `default` | `fonts` で登録した名前 |
| `size` | `48` | 文字の大きさ（`video.height` 基準の px） |
| `color` | `#ffffff` | 文字色 |
| `stroke.color` / `stroke.width` | | 縁取り。省略時は文字サイズから決まる |
| `max_chars_per_line` | `22` | 1行の目安の文字数。超えると文節の切れ目で折り返す |

吹き出しの色はキャラクターごとに `bubble.color` で決めます（[キャラクター](./characters.md)）。

## scenes

```yaml
scenes:
  - id: intro
    background: assets/slides/01.png   # 任意。このシーンだけの背景
    script:
      - ...
```

`id` は `daihon build --scene <id>` で指定するときに使います。

`script` には、セリフと演出命令を上から順に書きます。各要素の**最初のキー**で
種類が決まります。予約語（`show` `hide` `bg` `wait` `se` `note` `enter` `exit`）なら
演出命令、それ以外はキャラクター ID とみなしてセリフになります。

## セリフ

```yaml
- senpai: "台本はただのテキストだから、git で差分も見られるよ。"
  expr: idea        # 表情。指定した行から、次に指定するまで続く
  wait: 800         # この行の後の間（ms）。省略時は pause_between_lines_ms
  speed: 1.3        # 話す速さ。省略時はキャラの voice.speed
  read: "..."       # 音声エンジンに渡す読み。字幕には本文が出る
```

`read` は、字幕と読みを分けたいときに使います。英単語や固有名詞の読み違いを直すのが主な用途です。

```yaml
- senpai: "まずは daihon init で雛形を作ってみよう。"
  read: "まずは、ダイホン、イニットで雛形を作ってみよう。"
```

## 演出命令

| 命令 | 例 | 意味 |
|---|---|---|
| `show` | `- show: assets/x.png` | 画像を出す。`hide` かシーンの終わりまで残る |
| `hide` | `- hide: fig1` | `as` で名前を付けた画像を消す |
| `bg` | `- bg: assets/slides/03.png` | シーンの途中で背景を差し替える |
| `wait` | `- wait: 1200` | 何もせずに 1200ms 進める |
| `se` | `- se: assets/se/pon.mp3` | 効果音を鳴らす。時間は進めない |
| `note` | `- note: "あとで差し替える"` | 映像に影響しないメモ。`daihon check` が一覧表示する |
| `exit` | `- exit: kouhai` | キャラを退場させる |
| `enter` | `- enter: kouhai` | 退場中のキャラを、喋らせずに登場させる |

### show

```yaml
- show: assets/diagram/vpc.png
  as: fig1       # hide で消すときの名前
  at: center     # left | center | right。省略時は立ち絵の間に収まる
  enter: fade    # フェードインする
```

`center` の画像は、舞台にいる立ち絵と重ならない幅に縮められます。

### exit / enter

キャラは最初から全員舞台にいます。`exit` で退場させると、次に喋るか `enter` するまで描かれません。
喋れば自動で戻ってくるので、多くの場合 `exit` だけ書けば足ります。

```yaml
- exit: senpai
- kouhai: "……先輩、まだかな。"
- senpai: "お待たせ！"        # ここで登場する
```
