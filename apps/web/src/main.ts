const root = document.querySelector<HTMLElement>("#app");

if (!root) {
  throw new Error("scaffold root is missing");
}

root.dataset.phase = "0";
