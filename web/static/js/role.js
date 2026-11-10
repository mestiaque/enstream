// Host or viewer. The computer that runs ENStream (the "host") manages the library: folders,
// scans, PIN, backups, network. Phones, TVs and other computers are viewers: they watch, and
// their settings are only the ones that concern that device. The server enforces the same
// split (HOST_ONLY), so this only decides what the interface offers.
let host = true;

export const isHost = () => host;

export function setHost(value) {
  host = value !== false;
  document.body.classList.toggle("viewer", !host);
}
