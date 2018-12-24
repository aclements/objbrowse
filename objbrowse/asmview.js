// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

// TODO: Maybe get each piece with XHR.

// TODO: Separate out the overlays. Possibly make them generic so
// anything can feed rows with address ranges into them and they can
// add columns. Perhaps there should be a table abstraction that
// handles rows with AddrJS ranges and column groups for different
// overlays.
//
// Overlays: control flow, liveness, DWARF info for variables (might
// be better as an operand annotation that only appears for a selected
// operand), profiling info, data flow/aliasing (might be better as an
// operand annotation), extra information for resolved symbols (Go
// string or func object contents, offsets in global structures).

const ControlJump = 1
const ControlCall = 2
const ControlRet = 3
const ControlJumpUnknown = 4
const ControlExit = 5

class AsmView {
    constructor(data, container) {
        this._container = container;
        const view = this;
        const insts = data.Insts;

        // Create table.
        const table = $('<table class="disasm">').appendTo(container);
        this._table = table;

        // Create table header.
        const groupHeader = $("<thead>").appendTo(table).
              append($('<td colspan="5">'));
        const header = $("<thead>").appendTo(table).
              append($('<td colspan="5">'));
        const tableInfo = {table: table, groupHeader: groupHeader, header: header};

        // Create a zero-height TD at the top that will contain the
        // control flow arrows SVG.
        const arrowTD = $("<td>");
        $("<tr>").appendTo(table).
            append($('<td colspan="4">')).
            append(arrowTD);
        var arrowSVG;

        // Create disassembly table.
        const rows = [];
        const pcToRow = new Map();
        const pcRanges = [];
        const basePC = new AddrJS(insts[0].PC);
        for (var inst of insts) {
            const args = AsmView._formatArgs(inst.Args);
            const pc = new AddrJS(inst.PC);
            const pcDelta = pc.sub(basePC);
            // Create the row. The last TD is to extend the highlight over
            // the arrows SVG.
            const row = $("<tr>").
                  append($("<td>").text("0x"+inst.PC).addClass("pos")).
                  append($("<td>").text("+0x"+pcDelta).addClass("pos")).
                  append($("<td>").text(inst.Op).addClass("asm-inst")).
                  append($("<td>").append(args).addClass("asm-inst")).
                  append($("<td>")); // Extend the highlight over the arrows SVG
            table.append(row);

            const rowMeta = {elt: row, i: rows.length, width: 1, arrows: []};
            rows.push(rowMeta);
            pcToRow.set(inst.PC, rowMeta);
            pcRanges.push({start: pc, end: null, i: rowMeta.i, tr: row[0]});

            // Add a gap after strict block terminators.
            if (inst.Control.Type != 0) {
                if (inst.Control.Type != ControlCall && !inst.Control.Conditional)
                    table.append($("<tr>").css({height: "1em"}));
            }

            // On-click handler.
            row.click(() => {
                highlightRanges([pcRanges[rowMeta.i]], view);
            });
        }
        this._rows = rows;

        // Complete the PC ranges.
        for (let i = 1; i < pcRanges.length; i++) {
            pcRanges[i-1].end = pcRanges[i].start;
        }
        pcRanges[pcRanges.length-1].end = new AddrJS(data.LastPC);
        this._pcs = new IntervalMap(pcRanges);

        // Collect control-flow arrows.
        const arrows = [];
        for (var inst of insts) {
            if (inst.Control.Type == 0)
                continue;

            arrows.push({0: pcToRow.get(inst.PC),
                         1: pcToRow.get(inst.Control.TargetPC),
                         pos: 0, control: inst.Control});
        }

        // Sort arrows by length.
        function alen(arrow) {
            const r1 = arrow[0], r2 = arrow[1];
            if (!r2)
                return 0;
            return Math.abs(r1.i - r2.i);
        }
        arrows.sort(function(a, b) {
            return alen(a) - alen(b);
        })

        // Place the arrows.
        var cols = 1;
        for (var arrow of arrows) {
            const r1 = arrow[0], r2 = arrow[1];
            if (!r2)
                continue;

            var pos = 0;
            for (var i = Math.min(r1.i, r2.i); i <= Math.max(r1.i, r2.i); i++)
                pos = Math.max(pos, rows[i].width);
            arrow.pos = pos;

            for (var i = Math.min(r1.i, r2.i); i <= Math.max(r1.i, r2.i); i++)
                rows[i].width = pos + 1;
            cols = Math.max(cols, pos + 1);
        }

        // Draw arrows.
        const arrowWidth = 16;
        const indent = 8;
        const markerHeight = 8;
        if (arrows.length > 0) {
            const rowHeight = rows[0].elt.height();
            const svgWidth = cols * arrowWidth;
            arrowTD.css({"vertical-align": "top"});
            const arrowDiv = $("<div>").css({height: "0px"}).appendTo(arrowTD);
            const tdTop = arrowTD.offset().top;
            // Create the arrows SVG. This is absolutely positioned so the
            // row highlight in the other TRs can extend over it and has
            // pointer-events: none so hover passes through to the TR.
            arrowSVG = $(document.createElementNS("http://www.w3.org/2000/svg", "svg")).
                attr({height: table.height(), width: svgWidth}).
                css({"pointer-events": "none"}).
                appendTo(arrowDiv);
            for (var arrow of arrows) {
                const line = $(document.createElementNS("http://www.w3.org/2000/svg", "path"));
                line.appendTo(arrowSVG);

                const r1 = arrow[0], r2 = arrow[1];
                const y1 = r1.elt.offset().top - tdTop + rowHeight / 2;
                var marker = "url(#tri)";
                if (r2) {
                    // In-function arrow.
                    const x = arrow.pos * arrowWidth;;
                    const y2 = r2.elt.offset().top - tdTop + rowHeight / 2;
                    line.attr("d", "M" + x + " " + y1 +
                              " h" + indent +
                              " V" + y2 +
                              " h" + (-indent + markerHeight / 2));
                } else if (arrow.control.Type == ControlRet) {
                    // Exit arrow.
                    const y = r1.elt.offset().top - tdTop + rowHeight / 2;
                    const w = arrowWidth - markerHeight;
                    line.attr("d", "M " + (w + markerHeight) + " " + y + "h" + (-w));
                } else if (arrow.control.Type == ControlExit || arrow.control.TargetPC != 0 ||
                           (arrow.control.Type == ControlJump && arrow.control.TargetPC == 0)) {
                    // Out arrow.
                    // TODO: Some other arrow for dynamic target.
                    const y = r1.elt.offset().top - tdTop + rowHeight / 2;
                    line.attr("d", "M 0 " + y + "h" + (arrowWidth - markerHeight));
                    if (arrow.control.Type == ControlExit)
                        marker = "url(#markX)";
                }
                line.attr({stroke: "black", "stroke-width": "2px",
                           fill: "none", "marker-end": marker});

                // Attach the arrow to the outgoing and incoming
                // instructions.
                r1.arrows.push(line);
                if (r2)
                    r2.arrows.push(line);
            }
        }
        this._arrowSVG = arrowSVG;

        // Add liveness.
        if (data.Liveness)
            new LivenessOverlay(data.Liveness).render(tableInfo, this._pcs);
    }

    static _formatArgs(args) {
        const elts = [];
        var i = 0;
        for (var arg of args) {
            if (i++ > 0)
                elts.push(document.createTextNode(", "));

            var r;
            if (r = /([^+]*)(\+(0x)?[0-9]+)?\(SB\)/.exec(arg)) {
                const offset = parseInt(r[2]);
                const ranges = [{start: new AddrJS(offset),
                                 end: new AddrJS(offset+1)}];
                const url = "/s/" + r[1] + "#+" + formatRanges(ranges);
                elts.push($("<a>").attr("href", url).text(arg)[0]);
            } else {
                elts.push(document.createTextNode(arg))
            }
        }
        return $(elts);
    }

    highlightRanges(ranges, scroll) {
        // Clear row highlights.
        $(".highlight", this._table).removeClass("highlight");

        // Clear arrow highlights.
        $("path", this._arrowSVG).attr({stroke: "black"});

        // Highlight matching instructions.
        var first = true;
        for (let match of this._pcs.intersect(ranges)) {
            let rowMeta = this._rows[match.i];
            // Highlight row.
            rowMeta.elt.addClass("highlight");
            // Highlight arrows.
            rowMeta.arrows.forEach((a) => a.attr({stroke: "red"}));
            // Scroll in to view.
            if (first && scroll)
                scrollTo(this._container, rowMeta.elt);
            first = false;
            // TODO: Change color of markers (annoyingly hard without SVG 2)
        }
    }
}
