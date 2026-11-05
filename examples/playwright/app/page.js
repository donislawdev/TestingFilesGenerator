// The form's own check, in the browser, before anything is sent.

const limit = Number(document.body.dataset.limit);
const fixed = document.body.dataset.fixed === '1';

document.getElementById('upload').addEventListener('submit', async (event) => {
  event.preventDefault();
  const status = document.getElementById('status');
  const file = document.getElementById('file').files[0];
  // Mistake 4: a file of exactly the limit is refused. The limit is the
  // largest size allowed, so the comparison should be >.
  const tooLarge = fixed ? file.size > limit : file.size >= limit;
  if (tooLarge) {
    status.textContent = 'Refused (size_limit)';
    return;
  }
  const body = new FormData();
  body.append('file', file);
  const response = await fetch('/upload', { method: 'POST', body });
  const answer = await response.json();
  status.textContent = response.ok ? 'Uploaded' : `Refused (${answer.reason})`;
});
