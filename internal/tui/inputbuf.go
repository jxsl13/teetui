package tui

// TextInput is a rune buffer with a cursor and readline-style editing. It is the
// model behind every input line (chat, console, search); the view layer renders
// String() with the cursor at Cursor(). Pure and synchronous — it is only
// touched from the main event loop.
type TextInput struct {
	buf []rune
	cur int
}

// String returns the current text.
func (t *TextInput) String() string { return string(t.buf) }

// Cursor returns the cursor rune index (0..len).
func (t *TextInput) Cursor() int { return t.cur }

// SetString replaces the text and parks the cursor at the end.
func (t *TextInput) SetString(s string) {
	t.buf = []rune(s)
	t.cur = len(t.buf)
}

// Clear empties the buffer.
func (t *TextInput) Clear() {
	t.buf = t.buf[:0]
	t.cur = 0
}

// Insert adds a rune at the cursor.
func (t *TextInput) Insert(r rune) {
	t.buf = append(t.buf, 0)
	copy(t.buf[t.cur+1:], t.buf[t.cur:])
	t.buf[t.cur] = r
	t.cur++
}

// Backspace deletes the rune before the cursor.
func (t *TextInput) Backspace() {
	if t.cur == 0 {
		return
	}
	t.buf = append(t.buf[:t.cur-1], t.buf[t.cur:]...)
	t.cur--
}

// Delete removes the rune at the cursor (forward delete).
func (t *TextInput) Delete() {
	if t.cur >= len(t.buf) {
		return
	}
	t.buf = append(t.buf[:t.cur], t.buf[t.cur+1:]...)
}

// Left/Right/Home/End move the cursor.
func (t *TextInput) Left() {
	if t.cur > 0 {
		t.cur--
	}
}
func (t *TextInput) Right() {
	if t.cur < len(t.buf) {
		t.cur++
	}
}
func (t *TextInput) Home() { t.cur = 0 }
func (t *TextInput) End()  { t.cur = len(t.buf) }

// KillToEnd deletes from the cursor to the end (Ctrl-K).
func (t *TextInput) KillToEnd() {
	t.buf = t.buf[:t.cur]
}

// KillToStart deletes from the start to the cursor (Ctrl-U).
func (t *TextInput) KillToStart() {
	t.buf = append(t.buf[:0], t.buf[t.cur:]...)
	t.cur = 0
}

// KillWord deletes the whitespace-delimited word before the cursor (Ctrl-W).
func (t *TextInput) KillWord() {
	i := t.cur
	for i > 0 && t.buf[i-1] == ' ' {
		i--
	}
	for i > 0 && t.buf[i-1] != ' ' {
		i--
	}
	t.buf = append(t.buf[:i], t.buf[t.cur:]...)
	t.cur = i
}
