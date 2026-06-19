# Book PDF Fonts

Place exact book fonts here so PDF generation uses the same font inside Docker.

Expected cookbook font files:

- `GlacialIndifference-Regular.ttf`
- `GlacialIndifference-Bold.ttf`

The Docker image copies this directory to `/app/fonts` and sets `BOOK_FONT_DIR=/app/fonts`.
