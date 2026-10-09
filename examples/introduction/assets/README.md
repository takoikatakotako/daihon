# assets/

このサンプルの画像はすべて ChatGPT で生成したもので、リポジトリに同梱しています。
生成に使ったプロンプトは [`../PROMPTS.md`](../PROMPTS.md) にあります。
ライセンスはリポジトリ本体と同じ MIT です。

```
assets/
├── bg/
│   └── bg_outside_buildings.jpg
├── images/
│   ├── computer_screen_programming.png
│   ├── stopwatch.png
│   └── tv_video_hensyu_hard.png
├── senpai/       … 先輩（右）
│   ├── smile.png
│   └── idea.png
└── kouhai/       … 後輩（左）
    ├── smile.png
    ├── idea.png
    ├── shock.png
    ├── surprise.png
    └── question.png
```

**1キャラの画像はすべて同じキャンバスサイズにしてください。** 表情を切り替えたときに
位置がずれないようにするためです（このサンプルはすべて 704x1056）。

口パク用の差分は無いので、`characters/cast.yml` では `lipsync` を書いていません。

## BGM（任意）

`assets/bgm/bgm.mp3` に置き、`movie.yml` の `audio.bgm` のコメントを外すと流れます。
BGM は同梱していません。
