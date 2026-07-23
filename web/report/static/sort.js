// Column sorting for tables with class "sortable".  Clicking a header
// sorts by that column, numeric when the column parses as numbers, and
// clicking again reverses.  The active sort is kept in the URL fragment
// so the page's interval refresh preserves it.
(function () {
	"use strict";

	function cellKey(row, col) {
		var cell = row.cells[col];
		if (!cell) {
			return "";
		}
		var key = cell.getAttribute("data-key");
		if (key !== null && !isNaN(parseFloat(key))) {
			return parseFloat(key);
		}
		var text = cell.textContent.trim();
		if (/^-?[0-9][0-9.,]*$/.test(text)) {
			return parseFloat(text.replace(/,/g, ""));
		}
		return text.toLowerCase();
	}

	function sortTable(table, col, dir) {
		var body = table.tBodies[0];
		if (!body || body.rows.length < 2) {
			return;
		}
		var rows = Array.prototype.slice.call(body.rows, 1);
		rows.sort(function (a, b) {
			var ka = cellKey(a, col);
			var kb = cellKey(b, col);
			if (typeof ka === "number" && typeof kb === "number") {
				return dir * (ka - kb);
			}
			return dir * String(ka).localeCompare(String(kb));
		});
		rows.forEach(function (r) {
			body.appendChild(r);
		});
		var headers = body.rows[0].cells;
		for (var i = 0; i < headers.length; i++) {
			headers[i].classList.remove("sorted-asc", "sorted-desc");
		}
		headers[col].classList.add(dir > 0 ? "sorted-asc" : "sorted-desc");
	}

	function applyHash() {
		var m = /(?:^|[#&])sort=([\w-]+)\.(\d+)\.(-?1)/.exec(location.hash);
		if (!m) {
			return;
		}
		var table = document.getElementById(m[1]);
		if (table) {
			sortTable(table, parseInt(m[2], 10), parseInt(m[3], 10));
		}
	}

	document.addEventListener("DOMContentLoaded", function () {
		var tables = document.querySelectorAll("table.sortable");
		tables.forEach(function (table) {
			var header = table.rows[0];
			if (!header) {
				return;
			}
			Array.prototype.forEach.call(header.cells, function (th, col) {
				th.addEventListener("click", function () {
					var dir = th.classList.contains("sorted-asc") ? -1 : 1;
					sortTable(table, col, dir);
					location.hash = "sort=" + table.id + "." + col + "." + dir;
				});
			});
		});
		applyHash();
	});
})();
