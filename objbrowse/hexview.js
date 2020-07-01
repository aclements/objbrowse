// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

class HexView {
    constructor(data, container) {
        this._container = container;
        this._addr = new AddrJS(data.Addr);
        this._data = data;
        const view = this;

        // Construct string offsets index.
        this._offsets = this._makeOffsets();

        // Construct the interleaving of data lines and relocations.
        [this._rowMeta, this._rowIndex] = this._makeRowMeta();

        if (this._data.Data.length == 0) {
            $(container).text("(no data to display)");
        }

        // Create blocks for every 4K. It takes about 25ms to fill a
        // 4K block (most of which is the browser doing layout), which
        // is unnoticable.
        const table = $('<table>').appendTo(container);
        const blockBytes = 4 * 1024;
        const lines = this._rowMeta.length;
        const blockLines = blockBytes / 16;
        this._lazyTable = new LazyTable(table, lines, blockLines, this._makeRows.bind(this));
    }

    _makeRowMeta() {
        // Interleave rows for data and relocations.
        const data = this._data.Data;
        const relocs = this._data.Relocs;
        let rowMeta = [], rowIndex = [];
        let relI = 0;
        for (let i = 0; i < data.length / 2; i += 16) {
            rowIndex.push(rowMeta.length);
            rowMeta.push({off: i}); // Data offset
            for (; relI < relocs.length && relocs[relI].O < i + 16; relI++) {
                rowMeta.push({relI: relI, dataOff: i}); // Reloc index
            }
        }
        return [rowMeta, rowIndex];
    }

    _makeRows(startLine, nLines) {
        // TODO: Fetch data lazily from the server. Maybe get it in
        // bigger chunks and cache it. This might require the server
        // to do the data/reloc interleaving.
        const view = this;
        const rows = [];
        for (let rowI = startLine; rowI < startLine + nLines; rowI++) {
            // Create one row. Doing this directly with the DOM is
            // about 10X faster than doing it through jQuery.
            const tr = document.createElement("tr");
            const tdPos = document.createElement("td");
            tdPos.setAttribute("class", "pos");
            tr.appendChild(tdPos);
            const tdData = document.createElement("td");
            tr.appendChild(tdData);
            rows.push(tr);

            const rowMeta = this._rowMeta[rowI];
            if (rowMeta.off !== undefined) {
                // Data row.
                const rowAddr = this._addr.add(new AddrJS(rowMeta.off));

                tdPos.textContent = "0x" + rowAddr;
                tdData.setAttribute("class", "hv-data");
                tdData.textContent = this._formatLine(rowMeta.off);

                tr.addEventListener("click", () => {
                    const endAddr = rowAddr.add(new AddrJS(16));
                    highlightRanges([{start: rowAddr, end: endAddr}], view);
                });
            } else {
                // Relocation row.
                const reloc = this._data.Relocs[rowMeta.relI];

                tdData.appendChild(this._formatIndent(reloc.O - rowMeta.dataOff, reloc.B));
                tdData.appendChild(this._formatReloc(reloc));
            }
        }
        return rows;
    }

    // _formatLine returns one line of text representation of data,
    // starting at offset "start".
    _formatLine(start) {
        const dataLen = this._data.Data.length / 2;
        // Keep this in sync with _makeOffsets.
        let line = "";
        for (let i = 0; i < 16 && start + i < dataLen; i++) {
            if (i == 8)
                line += "  ";
            else if (i > 0)
                line += " ";
            line += this._data.Data.substr((start+i) * 2, 2);
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

    // _formatIndent returns a DOM node that highlights byte offsets
    // [start, start+length).
    _formatIndent(start, length) {
        // TODO: Right now this only highlights the beginning. We
        // should highlight the whole interval, but that may span rows
        // and make this more complicated. Maybe we instead want to
        // highlight the data rows directly and not indent the reloc
        // rows?

        if (start < 0) {
            var text = "< ";
        } else {
            const charOff = this._offsets[start][0];
            var text = " ".repeat(charOff) + "^ ";
        }
        const span = document.createElement("span");
        span.setAttribute("class", "hv-reloc-indent");
        span.textContent = text;
        return span;
    }

    // _formatReloc returns a DOM node with the formatted contents of
    // relocation reloc.
    _formatReloc(reloc) {
        const span = document.createElement("span");
        span.setAttribute("class", "hv-reloc");

        let text = this._data.RTypes[reloc.T];
        if (reloc.S == undefined) {
            // Keep it in one span.
            if (reloc.A !== undefined) {
                text += " " + reloc.A;
            }
            span.textContent = text;
            return span;
        }

        text += " ";
        span.appendChild(document.createTextNode(text));

        const a = document.createElement("a");
        span.appendChild(a);
        text = reloc.S;
        let url = "/s/" + reloc.S;
        if (reloc.A !== undefined) {
            text += " " + reloc.A;
            if (reloc.A >= 0) {
                const ranges = [{start: new AddrJS(reloc.A),
                                 end: new AddrJS(reloc.A+1)}];
                url += "#+" + formatRanges(ranges);
            }
        }
        a.setAttribute("href", url);
        a.textContent = text;
        return span;
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
                view._highlightTD(td, view._rowMeta[markLine].off, marks);
            }
            // Clear marks
            for (let i = 0; i < 16; i++)
                marks[i] = false;
            // Open the new line.
            markLine = newLine;
        }

        for (let r of ranges) {
            // Convert addresses into byte offsets.
            const b1 = r.start.sub(this._addr).toNumber();
            const b2 = r.end.sub(this._addr).toNumber();
            for (let b = b1; b < b2;) {
                const line = this._rowIndex[Math.floor(b / 16)];
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
