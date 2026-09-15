"use strict";
const airportExplorer = (() => {
  let data = null,
    error = "",
    query = "",
    region = "",
    offset = 0,
    loading = false,
    started = false,
    generation = 0,
    debounce;
  const limit = 25;
  const pretty = (value) => value.replaceAll("_", " ");
  function map() {
    const outline = [
      [-124.7, 48.5],
      [-123, 48.3],
      [-122.9, 49],
      [-95, 49],
      [-95, 48],
      [-89, 48],
      [-85, 46.7],
      [-83, 46.7],
      [-82, 43],
      [-79.8, 43.2],
      [-76.5, 44],
      [-71.5, 45],
      [-70, 47],
      [-67, 45],
      [-69.8, 43],
      [-74, 40.5],
      [-75.5, 38],
      [-75.7, 35.5],
      [-78, 34],
      [-80.5, 32],
      [-81, 29],
      [-80.2, 25.5],
      [-81, 25],
      [-82.6, 28],
      [-83, 29.5],
      [-85, 30],
      [-89.7, 29.3],
      [-93.8, 29.8],
      [-97.1, 26],
      [-99, 26.5],
      [-103, 29],
      [-106.5, 32],
      [-111, 31.3],
      [-114.7, 32.7],
      [-117, 32.5],
      [-120.5, 35],
      [-124.3, 42],
      [-124, 46],
    ];
    const mainland = (lng, lat) => [
      25 + ((lng + 126) / 60) * 790,
      20 + ((50 - lat) / 26) * 310,
    ];
    const pos = (a) =>
      a.region === "US-AK"
        ? [
            30 + (((a.lng > 0 ? a.lng - 360 : a.lng) + 190) / 60) * 235,
            405 + ((73 - a.lat) / 23) * 100,
          ]
        : a.region === "US-HI"
          ? [315 + ((a.lng + 161) / 7) * 190, 405 + ((23 - a.lat) / 5) * 100]
          : a.country !== "US"
            ? [
                590 + ((a.lng + 180) / 360) * 215,
                405 + ((25 - a.lat) / 45) * 100,
              ]
            : mainland(a.lng, a.lat);
    const path =
      outline
        .map((p, i) => (i ? "L" : "M") + mainland(...p).join(","))
        .join(" ") + "Z";
    return `<svg class="airport-map" viewBox="0 0 850 540" role="img" aria-label="Locations of ${data.matched} matching airports. Alaska, Hawaii and territories use separate scales. Use the searchable table for accessible airport details."><path d="${path}" class="us-outline"/><text x="30" y="355">CONTIGUOUS US</text>${[
      [20, "ALASKA"],
      [300, "HAWAII"],
      [580, "US TERRITORIES"],
    ]
      .map(
        ([x, label]) =>
          `<rect x="${x}" y="380" width="250" height="145" rx="8" class="airport-inset"/><text x="${x + 12}" y="399">${label} · SEPARATE SCALE</text>`,
      )
      .join("")}${data.map_airports
      .map((a) => {
        const [x, y] = pos(a);
        return `<circle cx="${x}" cy="${y}" r="${a.type === "large_airport" ? 4 : 2.5}" data-airport="${esc(a.ident)}" class="airport-point"><title>${esc(a.iata || a.ident)} · ${esc(a.name)}</title></circle>`;
      })
      .join("")}</svg>`;
  }
  function results() {
    if (!data)
      return error
        ? `<div class="error-banner" role="alert">${esc(error)} <button class="button small" data-airport-refresh>Retry connection</button></div>`
        : '<div class="loading-state"><span class="loader"></span>Connecting to the airport catalog…</div>';
    const fetched = new Date(data.fetched_at).toLocaleString();
    return `${error ? `<div class="error-banner" role="alert">${esc(error)} Showing the previous results.</div>` : ""}<div class="airport-summary"><div><strong>${num(data.total)}</strong><span>Airports in the source snapshot</span></div><div><strong>${num(data.matched)}</strong><span>Match your current filters</span></div><div><strong class="${data.stale ? "amber" : ""}">${data.stale ? "Cached · stale" : "Snapshot available"}</strong><span>Fetched ${esc(fetched)}</span></div></div><section class="panel"><div class="panel-header"><div><h2>Scheduled-service airport network</h2><p>All matching locations · Select a dot or a table row for details</p></div><span class="panel-tag">REAL REFERENCE DATA</span></div>${map()}<div class="table-foot">Illustrative outline; location coordinates from OurAirports. Insets use separate scales.</div></section><section class="panel airport-directory"><div class="panel-header"><div><h2>Airport directory</h2><p>Alphabetical by source identifier · ${data.matched ? offset + 1 : 0}–${Math.min(offset + limit, data.matched)} of ${num(data.matched)}</p></div><div class="actions"><button class="button small" data-airport-page="prev" ${offset === 0 || loading ? "disabled" : ""}>← Previous</button><button class="button small" data-airport-page="next" ${offset + limit >= data.matched || loading ? "disabled" : ""}>Next →</button></div></div>${data.airports.length ? `<div class="table-wrap" tabindex="0" aria-label="Airport results"><table><thead><tr><th>AIRPORT</th><th>CITY / REGION</th><th>TYPE</th><th>LOCATION</th></tr></thead><tbody>${data.airports.map((a) => `<tr><td><button class="text-link" data-airport="${esc(a.ident)}">${esc(a.name)} ↗</button><small class="airport-code">${esc(a.iata || "No IATA code")} / ${esc(a.ident)}</small></td><td>${esc(a.city || "Not specified")}<small class="airport-code">${esc(a.region)}</small></td><td>${esc(pretty(a.type))}</td><td class="mono">${a.lat.toFixed(3)}, ${a.lng.toFixed(3)}</td></tr>`).join("")}</tbody></table></div>` : '<div class="empty"><strong>No airports match.</strong>Try an airport code, city, name or a different region.</div>'}</section><p class="airport-source"><a href="https://ourairports.com/data/" target="_blank" rel="noreferrer">OurAirports public-domain data ↗</a> · ${esc(data.scope)}<br>These are airport locations, not CCO contracts, advertising screens, passenger counts or live flight data. ${data.stale ? esc(data.warning || "Snapshot is more than 48 hours old.") : ""}</p>`;
  }
  function render() {
    if (!started) {
      started = true;
      queueMicrotask(load);
    }
    return (
      heading(
        "Every airport. A clearer picture.",
        "Explore US scheduled-service airports through a dedicated Go data service.",
        '<button class="button" data-airport-refresh>↻ Reload catalog</button>',
        "AIRPORT INTELLIGENCE",
      ) +
      `<div id="airport-explorer"><div class="filters airport-filters"><label>Search airports<input id="airport-search" type="search" maxlength="100" placeholder="SEA, Atlanta, Anchorage…" value="${esc(query)}"></label><label>Region<select id="airport-region"><option value="">All US regions & territories</option>${(data?.regions || []).map((r) => `<option value="${esc(r)}" ${r === region ? "selected" : ""}>${esc(r)}</option>`).join("")}</select></label><span id="airport-loading" role="status">${loading ? "Loading…" : ""}</span></div><div id="airport-results">${results()}</div></div>`
    );
  }
  async function load() {
    const id = ++generation;
    loading = true;
    error = "";
    const status = document.querySelector("#airport-loading");
    if (status) status.textContent = "Loading…";
    try {
      const res = await request(
        "/api/v1/airports?" +
          new URLSearchParams({
            q: query,
            region,
            offset: String(offset),
            limit: String(limit),
          }),
      );
      if (id !== generation) return;
      data = res;
      offset = res.offset;
      const select = document.querySelector("#airport-region");
      if (select) {
        select.innerHTML =
          '<option value="">All US regions & territories</option>' +
          data.regions
            .map((r) => `<option value="${esc(r)}">${esc(r)}</option>`)
            .join("");
        select.value = region;
      }
    } catch (e) {
      if (id !== generation) return;
      error = e.message;
    } finally {
      if (id === generation) {
        loading = false;
        const el = document.querySelector("#airport-results");
        if (el) el.innerHTML = results();
        const status = document.querySelector("#airport-loading");
        if (status)
          status.textContent = error ? "Could not update results" : "";
      }
    }
  }
  document.addEventListener("input", (e) => {
    if (e.target.id !== "airport-search") return;
    query = e.target.value;
    offset = 0;
    ++generation;
    clearTimeout(debounce);
    debounce = setTimeout(load, 250);
  });
  document.addEventListener("change", (e) => {
    if (e.target.id !== "airport-region") return;
    region = e.target.value;
    offset = 0;
    clearTimeout(debounce);
    load();
  });
  document.addEventListener("click", (e) => {
    const refresh = e.target.closest("[data-airport-refresh]");
    if (refresh) {
      clearTimeout(debounce);
      load();
      return;
    }
    const page = e.target.closest("[data-airport-page]");
    if (page && !loading) {
      offset = Math.max(
        0,
        offset + (page.dataset.airportPage === "next" ? limit : -limit),
      );
      load();
      return;
    }
    const item = e.target.closest("[data-airport]");
    if (!item || !data) return;
    const a = data.map_airports.find((a) => a.ident === item.dataset.airport);
    if (!a) return;
    openDialog(
      `<div class="dialog-eyebrow">AIRPORT REFERENCE / ${esc(a.ident)}</div><h2>${esc(a.name)}</h2><dl class="detail-grid"><div><dt>IATA / SOURCE IDENTIFIER</dt><dd>${esc(a.iata || "Not assigned")} / ${esc(a.ident)}</dd></div><div><dt>CITY / REGION</dt><dd>${esc(a.city || "Not specified")} / ${esc(a.region)}</dd></div><div><dt>LOCATION</dt><dd>${a.lat}, ${a.lng}</dd></div><div><dt>FACILITY TYPE</dt><dd>${esc(pretty(a.type))}</dd></div></dl><p>Scheduled airline service according to the source snapshot. No advertising availability or passenger-volume claim is implied.</p><a class="button" href="https://ourairports.com/airports/${encodeURIComponent(a.ident)}/" target="_blank" rel="noreferrer">View source record ↗</a>`,
    );
  });
  return { render };
})();
