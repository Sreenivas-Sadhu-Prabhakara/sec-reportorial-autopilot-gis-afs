/* SEC reportorial calendar — local interface.
   Talks only to this binary on the loopback address. No other network calls. */
(function () {
  "use strict";

  // ───────── theme ─────────
  var root = document.documentElement;
  function storedTheme() { try { return localStorage.getItem("omsme-theme"); } catch (e) { return null; } }
  function currentTheme() {
    var t = root.getAttribute("data-theme");
    if (t) return t;
    return (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
  }
  function paintThemer(b) {
    if (!b) return;
    var dark = currentTheme() === "dark";
    b.textContent = dark ? "☀ Light" : "☾ Dark";
    b.setAttribute("aria-label", dark ? "Switch to light theme" : "Switch to dark theme");
  }
  var themer = document.querySelector("[data-themer]");
  if (themer) {
    paintThemer(themer);
    themer.addEventListener("click", function () {
      var next = currentTheme() === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("omsme-theme", next); } catch (e) { /* private window */ }
      paintThemer(themer);
    });
    if (window.matchMedia) {
      window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
        if (!storedTheme()) paintThemer(themer);
      });
    }
  }

  // ───────── small helpers ─────────
  function el(id) { return document.getElementById(id); }
  function h(tag, attrs) {
    var node = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        if (k === "class") node.className = attrs[k];
        else if (k === "text") node.textContent = attrs[k];
        else if (attrs[k] !== null && attrs[k] !== undefined) node.setAttribute(k, attrs[k]);
      });
    }
    for (var i = 2; i < arguments.length; i++) {
      var child = arguments[i];
      if (child === null || child === undefined || child === false) continue;
      node.appendChild(typeof child === "string" ? document.createTextNode(child) : child);
    }
    return node;
  }
  function clear(node) { while (node.firstChild) node.removeChild(node.firstChild); }

  var flashTimer = null;
  function flash(message, bad) {
    var box = el("flash");
    box.textContent = message;
    box.className = bad ? "flash bad" : "flash";
    box.hidden = false;
    if (flashTimer) window.clearTimeout(flashTimer);
    flashTimer = window.setTimeout(function () { box.hidden = true; }, 6000);
  }

  function api(path, options) {
    return fetch(path, options).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (body) {
        if (!res.ok) throw new Error(body.error || ("request failed: " + res.status));
        return body;
      });
    });
  }
  function send(method, path, body) {
    return api(path, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
  }

  var MONTHS = ["January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December"];

  var state = null;        // /api/state
  var selectedCompany = ""; // id of the company open on the Companies tab

  // ───────── tabs ─────────
  var PANELS = ["calendar", "companies", "filings", "settings", "rules"];
  document.querySelectorAll(".tabs button").forEach(function (button) {
    button.addEventListener("click", function () {
      var want = button.getAttribute("data-tab");
      document.querySelectorAll(".tabs button").forEach(function (b) {
        b.setAttribute("aria-pressed", String(b.getAttribute("data-tab") === want));
      });
      PANELS.forEach(function (name) { el("panel-" + name).hidden = name !== want; });
    });
  });

  // ───────── load ─────────
  function load() {
    return api("api/state").then(function (got) {
      state = got;
      el("data-file").textContent = got.dataFile;
      el("disclaimer").textContent = got.disclaimer;
      el("rules-disclaimer").textContent = got.disclaimer;
      el("rules-regnumber").textContent = got.registrationNumberNote;
      el("rules-penalty").textContent = got.penaltyNote;
      el("se-pen-note").textContent = got.penaltyNote;
      el("rules-asof").textContent =
        "Checked against the Commission's own issuances on " + got.scheduleAsOf +
        ". Everything here is indicative: confirm it before you file.";
      fillSelects();
      fillSettingsForm();
      renderRules();
      renderCompanies();
      renderFilings();
      return refreshCalendar();
    }).catch(function (err) { flash(err.message, true); });
  }

  function fillSelects() {
    function options(select, list, valueKey, labelKey) {
      var keep = select.value;
      clear(select);
      list.forEach(function (item) {
        select.appendChild(h("option", { value: item[valueKey] }, item[labelKey]));
      });
      if (keep) select.value = keep;
    }
    options(el("co-entity"), state.entityTypes, "value", "label");
    options(el("co-class"), state.filerClasses, "value", "label");
    options(el("co-basis"), state.gisBases, "value", "label");
    options(el("fi-req"), state.requirements, "value", "label");
    options(el("se-ov-req"), state.requirements, "value", "label");

    var months = el("co-fye-month");
    if (!months.options.length) {
      MONTHS.forEach(function (name, i) {
        months.appendChild(h("option", { value: String(i + 1) }, name));
      });
      months.value = "12";
    }

    [el("fi-company"), el("se-ov-company")].forEach(function (select, index) {
      var keep = select.value;
      clear(select);
      if (index === 1) select.appendChild(h("option", { value: "" }, "Every company"));
      state.companies.forEach(function (c) {
        select.appendChild(h("option", { value: c.id }, c.name));
      });
      if (keep) select.value = keep;
    });
  }

  // ───────── compliance calendar ─────────
  function tierClass(o) { return o.tier; }

  function refreshCalendar() {
    var year = el("cal-year").value;
    var asOf = el("cal-asof").value;
    var query = "?year=" + encodeURIComponent(year) + "&asOf=" + encodeURIComponent(asOf);
    el("cal-export").setAttribute("href", "api/export.csv" + query);
    return api("api/calendar" + query).then(function (got) {
      renderCounts(got.counts);
      renderCalendar(got.obligations);
    }).catch(function (err) { flash(err.message, true); });
  }

  function renderCounts(counts) {
    var box = el("cal-counts");
    clear(box);
    function count(label, value, kind) {
      box.appendChild(h("span", { class: "count" + (kind ? " " + kind : "") },
        h("b", { text: String(value) }), label));
    }
    count("obligations", counts.total);
    count("overdue", counts.overdue, counts.overdue ? "bad" : "");
    count("due within 7 days", counts.dueThisWeek, counts.dueThisWeek ? "warn" : "");
    count("filed", counts.filed, counts.filed ? "ok" : "");
    count("no deadline yet", counts.noDeadline);
  }

  function renderCalendar(list) {
    var body = el("cal-rows");
    clear(body);
    el("cal-empty").hidden = list.length > 0;
    list.forEach(function (o) {
      var due = o.known ? o.due : "not yet known";
      var row = h("tr", null,
        h("td", null, o.companyName),
        h("td", { class: "nowrap" }, o.requirement === "gis" ? "GIS" : shortOf(o.requirement)),
        h("td", { class: "when" }, due),
        h("td", null, h("span", { class: "badge " + tierClass(o) }, o.tierLabel)),
        h("td", { class: "act" }, o.action));
      body.appendChild(row);

      var why = h("td", { colspan: "5" });
      why.appendChild(h("p", { text: o.period + " — " + o.basis + (o.source ? " (" + o.source + ")" : "") }));
      if (o.safeDayNote) why.appendChild(h("p", { text: o.safeDayNote }));
      if (o.indicative && o.reason) why.appendChild(h("p", { text: o.reason }));
      if (o.overridden) why.appendChild(h("p", { text: "This date was entered by you, not shipped with the tool." }));
      body.appendChild(h("tr", { class: "why" }, why));
    });
  }

  function shortOf(requirement) {
    var found = "";
    state.requirements.forEach(function (r) { if (r.value === requirement) found = r.note; });
    return found || requirement;
  }

  el("cal-year").addEventListener("change", refreshCalendar);
  el("cal-asof").addEventListener("change", refreshCalendar);

  // ───────── companies ─────────
  function renderCompanies() {
    var body = el("co-rows");
    clear(body);
    el("co-empty").hidden = state.companies.length > 0;
    state.companies.forEach(function (c) {
      var open = h("button", { class: "linkish", type: "button" }, "Open");
      open.addEventListener("click", function () { openCompany(c.id); });
      var edit = h("button", { class: "linkish", type: "button" }, "Edit");
      edit.addEventListener("click", function () { fillCompanyForm(c); });
      var remove = h("button", { class: "linkish danger", type: "button" }, "Delete");
      remove.addEventListener("click", function () { deleteCompany(c); });

      body.appendChild(h("tr", null,
        h("td", null, c.name),
        h("td", { class: "when" }, c.registrationNumber || "—"),
        h("td", null, labelOf(state.entityTypes, c.entityType)),
        h("td", { class: "when" }, MONTHS[c.fiscalYearEndMonth - 1] + " " + c.fiscalYearEndDay),
        h("td", { class: "nowrap" }, open, " ", edit, " ", remove)));
    });
    if (selectedCompany && !state.companies.some(function (c) { return c.id === selectedCompany; })) {
      selectedCompany = "";
      el("co-detail").hidden = true;
    }
  }

  function labelOf(list, value) {
    var found = value;
    list.forEach(function (item) { if (item.value === value) found = item.label; });
    return found;
  }

  function fillCompanyForm(c) {
    el("co-form-title").textContent = c ? "Edit " + c.name : "Add a company";
    el("co-id").value = c ? c.id : "";
    el("co-name").value = c ? c.name : "";
    el("co-reg").value = c ? (c.registrationNumber || "") : "";
    el("co-entity").value = c ? c.entityType : "stock";
    el("co-class").value = c ? c.filerClass : "ordinary";
    el("co-fye-month").value = String(c ? c.fiscalYearEndMonth : 12);
    el("co-fye-day").value = String(c ? c.fiscalYearEndDay : 31);
    el("co-basis").value = c ? c.gisBasis : "meeting";
    el("co-reckon").value = c && c.reckoningDate ? c.reckoningDate : "";
    el("co-notes").value = c ? (c.notes || "") : "";
    paintBasisField();
    paintRegNote();
  }

  function paintBasisField() {
    var basis = el("co-basis").value;
    el("co-reckon-field").hidden = basis === "meeting";
  }

  function paintRegNote() {
    var digits = el("co-reg").value.replace(/\D/g, "");
    var note = state ? state.registrationNumberNote : "";
    el("co-reg-note").textContent = digits
      ? "Last digit " + digits.charAt(digits.length - 1) + ". " + note
      : note;
  }

  el("co-basis").addEventListener("change", paintBasisField);
  el("co-reg").addEventListener("input", paintRegNote);
  el("co-reset").addEventListener("click", function () { fillCompanyForm(null); });

  el("co-form").addEventListener("submit", function (event) {
    event.preventDefault();
    var id = el("co-id").value.trim();
    var body = {
      id: id || ("c" + Date.now().toString(36)),
      name: el("co-name").value.trim(),
      registrationNumber: el("co-reg").value.trim(),
      entityType: el("co-entity").value,
      filerClass: el("co-class").value,
      fiscalYearEndMonth: Number(el("co-fye-month").value),
      fiscalYearEndDay: Number(el("co-fye-day").value),
      gisBasis: el("co-basis").value,
      reckoningDate: el("co-reckon").value || null,
      notes: el("co-notes").value.trim()
    };
    send("PUT", "api/company", body).then(function () {
      flash("Saved " + body.name + ".");
      fillCompanyForm(null);
      return load().then(function () {
        if (selectedCompany === body.id) openCompany(body.id);
      });
    }).catch(function (err) { flash(err.message, true); });
  });

  function deleteCompany(c) {
    if (!window.confirm("Delete " + c.name + " and every filing recorded against it?")) return;
    send("DELETE", "api/company?id=" + encodeURIComponent(c.id)).then(function () {
      flash("Deleted " + c.name + ".");
      if (selectedCompany === c.id) { selectedCompany = ""; el("co-detail").hidden = true; }
      return load();
    }).catch(function (err) { flash(err.message, true); });
  }

  function openCompany(id) {
    selectedCompany = id;
    var query = "?id=" + encodeURIComponent(id) +
      "&year=" + encodeURIComponent(el("cal-year").value) +
      "&asOf=" + encodeURIComponent(el("cal-asof").value);
    api("api/company" + query).then(function (got) {
      el("co-detail").hidden = false;
      el("co-detail-name").textContent = got.company.name;
      el("co-detail-reg").textContent = got.registrationNumberNote;
      renderMeetings(got);
      renderObligations(got.obligations);
      renderDelinquency(got.delinquency);
      renderHistory(got.history);
      el("co-detail").scrollIntoView({ behavior: "smooth", block: "start" });
    }).catch(function (err) { flash(err.message, true); });
  }

  function renderMeetings(got) {
    var body = el("mt-rows");
    clear(body);
    var meetings = got.company.meetings || [];
    if (!meetings.length) {
      body.appendChild(h("tr", null, h("td", { colspan: "5" },
        "No meeting recorded. Until one is, there is no General Information Sheet deadline to state.")));
      return;
    }
    meetings.slice().reverse().forEach(function (m) {
      var remove = h("button", { class: "linkish danger", type: "button" }, "Remove");
      remove.addEventListener("click", function () {
        send("DELETE", "api/meeting?company=" + encodeURIComponent(got.company.id) + "&year=" + m.year)
          .then(function () { flash("Removed the " + m.year + " meeting."); return load(); })
          .then(function () { openCompany(got.company.id); })
          .catch(function (err) { flash(err.message, true); });
      });
      var due = "—";
      got.obligations.forEach(function (o) {
        if (o.requirement === "gis" && o.known && o.period.indexOf(m.date) >= 0) due = o.due;
      });
      body.appendChild(h("tr", null,
        h("td", { class: "when" }, String(m.year)),
        h("td", { class: "when" }, m.date),
        h("td", null, m.held ? "Held" : "Scheduled"),
        h("td", { class: "when" }, due),
        h("td", null, remove)));
    });
  }

  function renderObligations(list) {
    var box = el("co-obligations");
    clear(box);
    list.forEach(function (o) {
      var card = h("div", { class: "oblig " + tierClass(o) });
      card.appendChild(h("h5", null, o.requirement === "gis" ? "General Information Sheet" : shortOf(o.requirement),
        " — ", h("span", { class: "badge " + tierClass(o) }, o.tierLabel)));
      card.appendChild(h("p", { class: "lead", text: o.action }));
      card.appendChild(h("p", { text: o.period + " — " + o.basis + (o.source ? " (" + o.source + ")" : "") }));
      if (o.safeDayNote) card.appendChild(h("p", { text: o.safeDayNote }));
      if (o.reason) card.appendChild(h("p", { text: o.reason }));
      if (o.exposure && o.exposure.consequence) {
        card.appendChild(h("p", { text: o.exposure.label + ": " + o.exposure.consequence }));
      }
      if (o.exposure && o.exposure.amount) {
        card.appendChild(h("p", {
          text: o.exposure.amount.computed
            ? ("Your figures: ₱" + o.exposure.amount.total.toLocaleString() + ". " +
               o.exposure.amount.reason + (o.exposure.amount.suspensionNote ? " " + o.exposure.amount.suspensionNote : ""))
            : o.exposure.amount.reason
        }));
      }
      if (o.reminders && o.reminders.length) {
        var steps = h("div", { class: "steps-inline" });
        o.reminders.forEach(function (step) {
          steps.appendChild(h("span", { class: step.reached ? "reached" : "" }, step.label + " " + step.starts));
        });
        card.appendChild(steps);
      }
      box.appendChild(card);
    });
  }

  function renderDelinquency(watch) {
    var box = el("co-delinquency");
    clear(box);
    var cls = "watch" + (watch.reached ? " reached" : (watch.atRisk ? " risk" : ""));
    var card = h("div", { class: cls });
    card.appendChild(h("p", {
      text: watch.misses + " of " + watch.threshold + " missed filings in the five years to " + watch.windowTo + "."
        + (watch.reached
          ? " That is the statutory ground for the Commission to place the corporation under delinquent status."
          : (watch.atRisk ? " One more would be the statutory ground for delinquent status." : ""))
    }));
    if (watch.detail && watch.detail.length) {
      var list = h("ul");
      watch.detail.forEach(function (line) { list.appendChild(h("li", { text: line })); });
      card.appendChild(list);
    }
    card.appendChild(h("p", { class: "hint", text: watch.note }));
    box.appendChild(card);
  }

  function renderHistory(list) {
    var body = el("co-history");
    clear(body);
    list.forEach(function (o) {
      body.appendChild(h("tr", null,
        h("td", { class: "when" }, String(o.filingYear)),
        h("td", { class: "nowrap" }, shortOf(o.requirement)),
        h("td", { class: "when" }, o.known ? o.due : "—"),
        h("td", { class: "when" }, o.filedOn || "—"),
        h("td", null, o.reference || "—"),
        h("td", null, h("span", { class: "badge " + tierClass(o) }, o.exposure ? o.exposure.label : o.tierLabel))));
    });
  }

  el("mt-form").addEventListener("submit", function (event) {
    event.preventDefault();
    if (!selectedCompany) { flash("Open a company first.", true); return; }
    var date = el("mt-date").value;
    if (!date) { flash("The meeting needs a date.", true); return; }
    var body = { year: Number(date.slice(0, 4)), date: date, held: el("mt-held").value === "yes" };
    send("PUT", "api/meeting?company=" + encodeURIComponent(selectedCompany), body).then(function () {
      flash("Recorded the meeting of " + date + ".");
      el("mt-date").value = "";
      return load();
    }).then(function () { openCompany(selectedCompany); })
      .catch(function (err) { flash(err.message, true); });
  });

  // ───────── filings ─────────
  function renderFilings() {
    var body = el("fi-rows");
    clear(body);
    el("fi-empty").hidden = state.filings.length > 0;
    var names = {};
    state.companies.forEach(function (c) { names[c.id] = c.name; });
    state.filings.slice().sort(function (a, b) {
      return (b.filingYear - a.filingYear) || a.companyId.localeCompare(b.companyId);
    }).forEach(function (f) {
      var remove = h("button", { class: "linkish danger", type: "button" }, "Remove");
      remove.addEventListener("click", function () {
        send("DELETE", "api/filing?id=" + encodeURIComponent(f.id))
          .then(function () { flash("Removed the record."); return load(); })
          .then(function () { if (selectedCompany) openCompany(selectedCompany); })
          .catch(function (err) { flash(err.message, true); });
      });
      body.appendChild(h("tr", null,
        h("td", null, names[f.companyId] || f.companyId),
        h("td", { class: "nowrap" }, shortOf(f.requirement)),
        h("td", { class: "when" }, String(f.filingYear)),
        h("td", { class: "when" }, f.filedOn),
        h("td", null, f.reference + (f.channel ? " · " + f.channel : "")),
        h("td", null, remove)));
    });
  }

  el("fi-form").addEventListener("submit", function (event) {
    event.preventDefault();
    var body = {
      id: "f" + Date.now().toString(36),
      companyId: el("fi-company").value,
      requirement: el("fi-req").value,
      filingYear: Number(el("fi-year").value),
      filedOn: el("fi-date").value,
      reference: el("fi-ref").value.trim(),
      channel: el("fi-channel").value.trim(),
      filedBy: el("fi-by").value.trim(),
      note: el("fi-note").value.trim()
    };
    send("PUT", "api/filing", body).then(function () {
      flash("Recorded the filing.");
      el("fi-ref").value = "";
      el("fi-note").value = "";
      return load();
    }).then(function () { if (selectedCompany) openCompany(selectedCompany); })
      .catch(function (err) { flash(err.message, true); });
  });

  // ───────── settings ─────────
  var holidays = {};
  var overrides = [];

  function fillSettingsForm() {
    var s = state.settings;
    el("se-plan").value = s.reminderLadder.planDays;
    el("se-prepare").value = s.reminderLadder.prepareDays;
    el("se-soon").value = s.reminderLadder.soonDays;
    el("se-urgent").value = s.reminderLadder.urgentDays;
    el("se-final").value = s.reminderLadder.finalDays;
    el("se-pen-source").value = s.penaltyFigures.source || "";
    el("se-pen-late").value = s.penaltyFigures.basicLate || "";
    el("se-pen-non").value = s.penaltyFigures.basicNonFiling || "";
    el("se-pen-monthly").value = s.penaltyFigures.monthlyRate || "";
    holidays = Object.assign({}, s.holidays || {});
    overrides = (s.deadlineOverrides || []).slice();
    renderHolidays();
    renderOverrides();
  }

  function renderHolidays() {
    var body = el("se-holidays");
    clear(body);
    var days = Object.keys(holidays).sort();
    if (!days.length) {
      body.appendChild(h("tr", null, h("td", { colspan: "3" }, "None recorded. Weekends are always treated as non-working days.")));
      return;
    }
    days.forEach(function (day) {
      var remove = h("button", { class: "linkish danger", type: "button" }, "Remove");
      remove.addEventListener("click", function () { delete holidays[day]; renderHolidays(); });
      body.appendChild(h("tr", null,
        h("td", { class: "when" }, day),
        h("td", null, holidays[day]),
        h("td", null, remove)));
    });
  }

  function renderOverrides() {
    var body = el("se-overrides");
    clear(body);
    if (!overrides.length) {
      body.appendChild(h("tr", null, h("td", { colspan: "6" }, "None. The calendar uses the dated schedule this build carries.")));
      return;
    }
    var names = {};
    state.companies.forEach(function (c) { names[c.id] = c.name; });
    overrides.forEach(function (o, index) {
      var remove = h("button", { class: "linkish danger", type: "button" }, "Remove");
      remove.addEventListener("click", function () { overrides.splice(index, 1); renderOverrides(); });
      body.appendChild(h("tr", null,
        h("td", { class: "nowrap" }, shortOf(o.requirement)),
        h("td", { class: "when" }, String(o.filingYear)),
        h("td", null, o.companyId ? (names[o.companyId] || o.companyId) : "Every company"),
        h("td", { class: "when" }, o.due),
        h("td", null, o.source),
        h("td", null, remove)));
    });
  }

  el("se-hol-add").addEventListener("click", function () {
    var day = el("se-hol-date").value;
    if (!day) { flash("A non-working day needs a date.", true); return; }
    holidays[day] = el("se-hol-name").value.trim() || "non-working day";
    el("se-hol-date").value = "";
    el("se-hol-name").value = "";
    renderHolidays();
    flash("Added. Save the settings to keep it.");
  });

  el("se-ov-add").addEventListener("click", function () {
    var due = el("se-ov-due").value;
    var source = el("se-ov-source").value.trim();
    if (!due || !source) { flash("A deadline needs both the date and the issuance that set it.", true); return; }
    overrides.push({
      requirement: el("se-ov-req").value,
      filingYear: Number(el("se-ov-year").value),
      companyId: el("se-ov-company").value,
      due: due,
      source: source,
      note: ""
    });
    el("se-ov-due").value = "";
    el("se-ov-source").value = "";
    renderOverrides();
    flash("Added. Save the settings to keep it.");
  });

  el("se-save").addEventListener("click", function () {
    var body = {
      holidays: holidays,
      reminderLadder: {
        planDays: Number(el("se-plan").value),
        prepareDays: Number(el("se-prepare").value),
        soonDays: Number(el("se-soon").value),
        urgentDays: Number(el("se-urgent").value),
        finalDays: Number(el("se-final").value)
      },
      deadlineOverrides: overrides,
      penaltyFigures: {
        source: el("se-pen-source").value.trim(),
        enteredOn: todayISO(),
        basicLate: Number(el("se-pen-late").value || 0),
        basicNonFiling: Number(el("se-pen-non").value || 0),
        monthlyRate: Number(el("se-pen-monthly").value || 0)
      }
    };
    send("PUT", "api/settings", body).then(function () {
      flash("Settings saved.");
      return load();
    }).then(function () { if (selectedCompany) openCompany(selectedCompany); })
      .catch(function (err) { flash(err.message, true); });
  });

  // ───────── rules ─────────
  function renderRules() {
    var body = el("rules-rows");
    clear(body);
    state.rules.forEach(function (r) {
      body.appendChild(h("tr", null,
        h("td", null, r.rule),
        h("td", null, r.value),
        h("td", null, r.source)));
    });

    var schedule = el("rules-schedule");
    clear(schedule);
    state.schedule.forEach(function (row) {
      var where = row.source;
      if (row.supersededDue) where += " — first set at " + row.supersededDue + " by " + row.supersededBy;
      schedule.appendChild(h("tr", null,
        h("td", { class: "nowrap" }, shortOf(row.requirement)),
        h("td", { class: "when" }, row.fiscalYearEnd),
        h("td", { class: "when" }, row.due),
        h("td", null, where)));
    });
  }

  // ───────── start ─────────
  function todayISO() {
    var now = new Date();
    return now.getFullYear() + "-" +
      String(now.getMonth() + 1).padStart(2, "0") + "-" +
      String(now.getDate()).padStart(2, "0");
  }

  el("cal-asof").value = todayISO();
  el("cal-year").value = String(new Date().getFullYear());
  el("fi-year").value = String(new Date().getFullYear());
  el("se-ov-year").value = String(new Date().getFullYear() + 1);
  fillCompanyForm(null);
  load();
})();
