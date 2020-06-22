// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

class HexView {
    constructor(data, container) {
        this._container = container;
        this._addr = new AddrJS(data.Addr);
        this._data = data.Data;
        const view = this;

        // Construct string offsets index.
        this._offsets = this._makeOffsets();

        // Create blocks for every 4K. It takes about 25ms to fill a
        // 4K block (most of which is the browser doing layout), which
        // is unnoticable.
        const table = $('<table>').appendTo(container);
        const blockBytes = 4 * 1024;
        const length = data.Data.length / 2;
        const lines = Math.ceil(length / 16);
        const blockLines = blockBytes / 16;
        this._lazyTable = new LazyTable(table, lines, blockLines, this._makeRows.bind(this));
    }

    _makeRows(startLine, nLines) {
        // TODO: Fetch data lazily from the server. Maybe get it in
        // bigger chunks and cache it.
        const view = this;
        const start = startLine * 16;
        const length = nLines * 16;
        const dataLen = this._data.length / 2;
        const rows = [];
        let rowAddr = this._addr.add(new AddrJS(start));
        for (let off = start; off < start+length; off += 16) {
            // Create one row. Doing this directly with the DOM is
            // about 10X faster than doing it through jQuery.
            const tr = document.createElement("tr");
            const tdPos = document.createElement("td");
            tdPos.setAttribute("class", "pos");
            tdPos.textContent = "0x" + rowAddr;
            tr.appendChild(tdPos);
            const tdData = document.createElement("td");
            tdData.setAttribute("class", "hv-data");
            tdData.textContent = this._formatLine(off);
            tr.appendChild(tdData);

            const startAddr = rowAddr;
            const nextAddr = this._addr.add(new AddrJS(off + 16));
            tr.addEventListener("click", () => {
                highlightRanges([{start: startAddr, end: nextAddr}], view);
            });
            rowAddr = nextAddr;

            rows.push(tr);
        }
        return rows;
    }

    // _formatLine returns one line of text representation of data,
    // starting at offset "start".
    _formatLine(start) {
        const dataLen = this._data.length / 2;
        // Keep this in sync with _makeOffsets.
        let line = "";
        for (let i = 0; i < 16 && start + i < dataLen; i++) {
            if (i == 8)
                line += "  ";
            else if (i > 0)
                line += " ";
            line += this._data.substr((start+i) * 2, 2);
        }
        return line;
    }

    // _makeOffsets string offsets of each byte in a formatted line.
    _makeOffsets() {
        const offsets = [];
        let offset = 0;
        for (let i = 0; i < 16; i++) {
            if (i == 8)
                offset += 2;
            else if (i > 0)
                offset += 1;
            const start = offset;
            offset += 2;
            offsets.push([start, offset]);
        }
        offsets.push([offset]);
        offsets[-1] = [undefined, 0];
        return offsets;
    }

    // _highlightTD highlights the bytes in data TD "td" according to
    // the boolean vector "marks". The data in TD must start at
    // "start".
    _highlightTD(td, start, marks) {
        const line = this._formatLine(start);
        td.textContent = "";    // Clear TD
        for (let i = 0, j = 0; i < marks.length; i = j) {
            // Find the end of the mark run.
            for (j = i + 1; j < marks.length && marks[i] == marks[j]; j++) {}

            // Mark [i, j) run.
            if (marks[i]) {
                const subline = line.substring(
                    this._offsets[i][0],
                    this._offsets[j-1][1]
                );
                const span = document.createElement("span");
                span.setAttribute("class", "highlight");
                span.textContent = subline;
                td.appendChild(span);
            } else {
                const subline = line.substring(
                    this._offsets[i-1][1],
                    this._offsets[j][0]
                )
                td.appendChild(document.createTextNode(subline));
            }
        }
    }

    highlightRanges(ranges, scroll) {
        // Clear highlights.
        $(".highlight", this._lazyTable.tableElt).removeClass("highlight");

        // Mark ranges. This is tricky because ranges may only
        // partially overlap lines and multiple ranges might overlap
        // the same line. Hence, we accumulate marks one line at a
        // time and flush them out when we move to a different line.
        let firstTR = undefined;
        let markLine = undefined;
        const marks = [];
        const view = this;
        function getDataTD(tr) {
            return tr.childNodes[1]
        }
        function openLine(newLine) {
            if (markLine === newLine)
                return;
            if (markLine !== undefined) {
                // Flush marks for markLine.
                const tr = view._lazyTable.getRow(markLine);
                if (firstTR === undefined)
                    firstTR = tr;
                const td = getDataTD(tr);
                view._highlightTD(td, markLine * 16, marks);
            }
            // Clear marks
            for (let i = 0; i < 16; i++)
                marks[i] = false;
            // Open the new line.
            markLine = newLine;
        }

        const fullMarks = [];
        for (let i = 0; i < 16; i++)
            fullMarks[i] = true;

        for (let r of ranges) {
            // Convert addresses into byte offsets.
            const b1 = r.start.sub(this._addr).toNumber();
            const b2 = r.end.sub(this._addr).toNumber();
            for (let b = b1; b < b2;) {
                const line = Math.floor(b / 16);
                openLine(line);
                marks[b % 16] = true;
                b++;
            }
        }
        // Flush final line.
        openLine(undefined);

        // Scroll.
        if (firstTR && scroll)
            scrollTo(this._container, firstTR);
    }
}
