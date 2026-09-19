# Offline speech dependency

`build-ascii.ps1` downloads and verifies `sherpa-onnx-1.13.8-verified.aar`.
The Gradle `verifySenseVoiceDependency` task verifies it again on every build.

- Upstream: `https://github.com/k2-fsa/sherpa-onnx`
- Release: `v1.13.8`
- Artifact: `sherpa-onnx-1.13.8.aar`
- Bytes: `50129134`
- SHA-256: `633c24321e06b1fe79feafa03ea16cbc0f8a286641e2da3559bac91bdb13bd96`
- sherpa-onnx license: Apache-2.0; the AAR also contains its upstream native dependencies.

SenseVoice model files are downloaded only by the build pipeline, verified and
embedded in the APK. The phone extracts bundled assets without using a network.
Fixed upstream repository:
`csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17`,
revision `2365baeacb507f821a0c8120fcee3d484dba7a07`.
The model and tokenizer checksums are pinned in `SenseVoiceModelManager`.

Audio stays on the device with SenseVoice. A working system recognition service
is used first; the legacy Vosk UI and download entry are removed.
No claim of device-specific latency,
accuracy improvement, or memory measurements is made without a phone test.
