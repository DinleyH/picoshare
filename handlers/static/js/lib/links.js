export function makeShortLink(fileId) {
  return `${window.location.origin}/-${fileId}`;
}

export function makePlaylistShortLink(playlistId) {
  return `${window.location.origin}/pl-${playlistId}`;
}

export function makeVerboseLink(fileId, filename) {
  return makeShortLink(fileId) + "/" + encodeURIComponent(filename);
}
