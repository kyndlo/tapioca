# Model freshness and support policy

Before changing the model catalog, runtime pins, platform defaults, or model
recommendations, research the current ecosystem on the day of the change.

- Search Hugging Face model pages and the Hub API for recently released and
  trending image, audio/speech, video, and language models.
- Search the relevant AI communities on Reddit for field reports, especially
  r/LocalLLaMA and r/LocalLLM for language models and agents,
  r/StableDiffusion and r/comfyui for image/video models, and relevant audio AI
  communities for speech models.
- Use Reddit as a discovery and real-hardware signal, not as the source of
  truth. Verify model identity, files, license, architecture, and documented
  runtime support against the official Hugging Face model card and upstream
  runtime or model repository.
- Prefer the newest model that is compatible and testable over a merely newer
  upload. Do not add a catalog entry until its exact artifacts exist, its
  license and access requirements are represented, the pinned backend can load
  it, hardware guidance is documented, and resolution/validation tests pass.
- Include the research date and the important Hugging Face, upstream, and
  Reddit links in the change summary so the decision can be audited later.

The daily discovery workflow is `.github/workflows/model-freshness.yml`. Keep
its source categories and compatibility checks aligned with this policy when
the catalog or supported modality set changes. Preserve its separated primary
and fallback schedules: GitHub scheduled workflows are best-effort, and the
fallback prevents a delayed runner allocation from leaving a full day without
a report. Do not add a shared concurrency group that can couple the two runs.
