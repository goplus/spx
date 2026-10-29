# Generated audio fixture

`tone.mp3` is a synthesized 440 Hz, mono, 22,050 Hz, 0.1-second tone.
It contains no third-party recording. Reproduce with:

```sh
ffmpeg -f lavfi -i 'sine=frequency=440:sample_rate=22050:duration=0.1' \
  -ac 1 -codec:a libmp3lame -b:a 32k -map_metadata -1 \
  -write_xing 0 -id3v2_version 0 tone.mp3
```
