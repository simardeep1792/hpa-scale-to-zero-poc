const form = document.querySelector("#lag-form");
const submit = form.querySelector("button");
const message = document.querySelector("#message");

function setMessage(text) {
  message.textContent = text;
}

async function refresh() {
  try {
    const response = await fetch("/api/state", { cache: "no-store" });
    if (!response.ok) throw new Error("state request failed");
    const state = await response.json();
    document.querySelector("#queue-lag").textContent = `${state.queueLag} tasks`;
    document.querySelector("#worker-replicas").textContent = `${state.workerReady} ready / ${state.workerDesired} desired`;
    document.querySelector("#scaled-zero").textContent = state.scaledToZero || "Reconciling";
    const selected = form.querySelector(`input[value="${state.queueLag}"]`);
    if (selected) selected.checked = true;
  } catch {
    setMessage("Unable to read cluster state. Keep this tunnel open and try again.");
  }
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const lag = new FormData(form).get("lag");
  submit.disabled = true;
  setMessage("Submitting queue lag update.");
  try {
    const response = await fetch("/api/lag", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ lag }),
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.message || "update failed");
    setMessage(result.message);
    window.setTimeout(refresh, 2000);
  } catch (error) {
    setMessage(error.message || "Unable to update queue lag.");
  } finally {
    submit.disabled = false;
  }
});

refresh();
window.setInterval(refresh, 5000);
