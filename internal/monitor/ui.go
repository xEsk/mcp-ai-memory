package monitor

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MCP Memory Monitor</title>
  <style>
    :root {
      color-scheme: light dark;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f7f7f4;
      color: #202124;
    }
    body {
      margin: 0;
      min-height: 100vh;
      background: #f7f7f4;
    }
    header {
      position: sticky;
      top: 0;
      z-index: 2;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      padding: 14px 20px;
      border-bottom: 1px solid #d8d6cf;
      background: #ffffff;
    }
    h1 {
      margin: 0;
      font-size: 18px;
      font-weight: 650;
    }
    .status {
      font-size: 13px;
      color: #4f5b54;
    }
    main {
      padding: 16px 20px 28px;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      table-layout: fixed;
      background: #ffffff;
      border: 1px solid #d8d6cf;
      border-radius: 6px;
      overflow: hidden;
    }
    th, td {
      padding: 10px 12px;
      border-bottom: 1px solid #eceae4;
      text-align: left;
      vertical-align: top;
      font-size: 13px;
    }
    th {
      background: #efeee8;
      font-weight: 650;
      color: #30342f;
    }
    tr:last-child td {
      border-bottom: 0;
    }
    .time { width: 180px; }
    .tool { width: 160px; }
    .phase { width: 90px; }
    .duration { width: 90px; }
    .session { width: 190px; }
    code, pre {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 12px;
    }
    pre {
      max-height: 180px;
      overflow: auto;
      white-space: pre-wrap;
      word-break: break-word;
      margin: 0;
      padding: 8px;
      background: #f3f5f1;
      border: 1px solid #dfe3db;
      border-radius: 4px;
    }
    .error {
      color: #9b1c1c;
      font-weight: 650;
    }
    @media (prefers-color-scheme: dark) {
      :root, body { background: #171917; color: #eef0ea; }
      header, table { background: #20231f; border-color: #3a4038; }
      th { background: #2a2e29; color: #f5f5ef; }
      th, td { border-bottom-color: #343a32; }
      .status { color: #bbc7b9; }
      pre { background: #171917; border-color: #3a4038; }
      .error { color: #ff9d9d; }
    }
  </style>
</head>
<body>
  <header>
    <h1>MCP Memory Monitor</h1>
    <div class="status" id="status">connecting...</div>
  </header>
  <main>
    <table>
      <thead>
        <tr>
          <th class="time">Time</th>
          <th class="session">Session</th>
          <th class="tool">Tool</th>
          <th class="phase">Phase</th>
          <th class="duration">Duration</th>
          <th>Payload</th>
        </tr>
      </thead>
      <tbody id="events"></tbody>
    </table>
  </main>
  <script>
    const tbody = document.getElementById("events");
    const status = document.getElementById("status");
    const source = new EventSource("/events/stream");
    const rows = [];

    source.onopen = () => status.textContent = "live";
    source.onerror = () => status.textContent = "reconnecting...";
    source.onmessage = (message) => {
      const event = JSON.parse(message.data);
      rows.unshift(event);
      if (rows.length > 200) rows.pop();
      render();
    };

    function render() {
      tbody.innerHTML = rows.map(event => {
        const payload = {
          arguments: event.arguments,
          response: event.response,
          error: event.error
        };
        return "<tr>" +
          "<td>" + escapeText(formatTime(event.timestamp)) + "</td>" +
          "<td><code>" + escapeText(event.session_id || "") + "</code><br><code>pid " + escapeText(String(event.pid || "")) + "</code></td>" +
          "<td><code>" + escapeText(event.tool || "") + "</code></td>" +
          "<td class=\"" + (event.phase === "error" ? "error" : "") + "\">" + escapeText(event.phase || "") + "</td>" +
          "<td>" + (event.duration_ms ? escapeText(String(event.duration_ms)) + " ms" : "") + "</td>" +
          "<td><pre>" + escapeText(JSON.stringify(payload, null, 2)) + "</pre></td>" +
        "</tr>";
      }).join("");
    }

    function formatTime(value) {
      if (!value) return "";
      return new Date(value).toLocaleString();
    }

    function escapeText(value) {
      return value.replace(/[&<>"']/g, char => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;",
        "'": "&#39;"
      }[char]));
    }
  </script>
</body>
</html>`
