// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

const ControlJump = 1
const ControlCall = 2
const ControlRet = 3
const ControlExit = 4

function highlightRanges(ranges) {
    disasmHighlightRanges(ranges);
    sourceView.highlightRanges(ranges);
}

// IntervalMap is a map of intervals.
class IntervalMap {
    // ranges must be an array of [low, high, ...] arrays, where ...
    // can be any additional data the caller wishes to store.
    constructor(ranges) {
        // Sort ranges.
        ranges.sort((a, b) => compareAddr(a[0], b[0]));
        this.ranges = ranges;
    }

    // Intersect a list of ranges with the ranges in this map and
    // return the subset that overlap. ranges must already be sorted.
    intersect(ranges) {
        const out = [];
        let i = 0, j = 0;
        while (i < ranges.length && j < this.ranges.length) {
            if (IntervalMap.overlap(ranges[i], this.ranges[j])) {
                // Found a match.
                out.push(this.ranges[j]);
                j++;
            } else if (ranges[i][0] < this.ranges[j][0]) {
                i++;
            } else {
                j++;
            }
        }
        return out;
    }

    static overlap(r1, r2) {
        return compareAddr(r1[1], r2[0]) > 0 && compareAddr(r1[0], r2[1]) < 0;
    }
}

// Compare two AddrJS values.
function compareAddr(a, b) {
    if (a.length != b.length) {
        return a.length - b.length;
    }
    if (a < b)
        return -1;
    else if (a > b)
        return 1;
    return 0;
}

var disasmTable;
var disasmRows;
var disasmPCs; // IntervalMap to row indexes;
var disasmArrowSVG;
var sourceView;

function disasm(container, info) {
    const insts = info.Insts;

    // Create table.
    const table = $('<table class="disasm">').appendTo($(container).empty());
    disasmTable = table;

    // Create table header.
    const groupHeader = $("<thead>").appendTo(table).
          append($('<td colspan="4">'));
    const header = $("<thead>").appendTo(table).
          append($('<td colspan="4">'));
    const tableInfo = {table: table, groupHeader: groupHeader, header: header};

    // Create a zero-height TD at the top that will contain the
    // control flow arrows SVG.
    const arrowTD = $("<td>");
    $("<tr>").appendTo(table).
        append($('<td colspan="3">')).
        append(arrowTD);
    var arrowSVG;

    // Create disassembly table.
    const rows = [];
    const pcToRow = new Map();
    const pcRanges = [];
    for (var inst of insts) {
        const args = formatArgs(inst.Args);
        // Create the row. The last TD is to extend the highlight over
        // the arrows SVG.
        const row = $("<tr>").attr("tabindex", -1).
              append($("<td>").text("0x"+inst.PC)).
              append($("<td>").text(inst.Op)).
              append($("<td>").append(args)).
              append($("<td>")); // Extend the highlight over the arrows SVG
        table.append(row);

        const rowMeta = {elt: row, i: rows.length, width: 1, arrows: []};
        rows.push(rowMeta);
        pcToRow.set(inst.PC, rowMeta);
        pcRanges.push([inst.PC, 0, rowMeta.i]);

        // Add a gap after strict block terminators.
        if (inst.Control.Type != 0) {
            if (inst.Control.Type != ControlCall && !inst.Control.Conditional)
                table.append($("<tr>").css({height: "1em"}));
        }

        // On-click handler.
        row.click(() => {
            highlightRanges([pcRanges[rowMeta.i]]);
        });
    }
    disasmRows = rows;

    // Complete the PC ranges.
    for (let i = 1; i < pcRanges.length; i++) {
        pcRanges[i-1][1] = pcRanges[i][0];
    }
    pcRanges[pcRanges.length-1][1] = info.LastPC;
    disasmPCs = new IntervalMap(pcRanges);

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
        arrowTD.css({"vertical-align": "top", "width": svgWidth});
        const tdTop = arrowTD.offset().top;
        // Create the arrows SVG. This is absolutely positioned so the
        // row highlight in the other TRs can extend over it and has
        // pointer-events: none so hover passes through to the TR.
        arrowSVG = $(document.createElementNS("http://www.w3.org/2000/svg", "svg")).
            attr({height: table.height(), width: svgWidth}).
            css({position: "absolute", "pointer-events": "none"}).
            appendTo(arrowTD);
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
            } else if (arrow.control.Type == ControlExit || arrow.control.TargetPC != 0) {
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
    disasmArrowSVG = arrowSVG;

    // Add liveness.
    renderLiveness(info, tableInfo, rows);

    // Add source view.
    sourceView = new SourceView(info.SourceView, container);
}

function formatArgs(args) {
    const elts = [];
    var i = 0;
    for (var arg of args) {
        if (i++ > 0)
            elts.push(document.createTextNode(", "));

        var r;
        if (r = /([^+]*)(\+(0x)?[0-9]+)?\(SB\)/.exec(arg)) {
            // TODO: Link to offset if there is one.
            elts.push($("<a>").attr("href", "/s/" + r[1]).text(arg)[0]);
        } else {
            elts.push(document.createTextNode(arg))
        }
    }
    return $(elts);
}

function disasmHighlightRanges(ranges) {
    // Clear row highlights.
    $(".highlight", disasmTable).removeClass("highlight");

    // Clear arrow highlights.
    $("path", disasmArrowSVG).attr({stroke: "black"});

    // Highlight matching instructions.
    for (let match of disasmPCs.intersect(ranges)) {
        let rowMeta = disasmRows[match[2]];
        // Highlight row.
        rowMeta.elt.addClass("highlight");
        // Highlight arrows.
        rowMeta.arrows.forEach((a) => a.attr({stroke: "red"}));
        // TODO: Change color of markers (annoyingly hard without SVG 2)
    }
}

function renderLiveness(info, table, rows) {
    const ptrSize = info.Liveness.PtrSize;

    // Decode live bitmaps.
    const locals = [], args = [];
    for (const bm of info.Liveness.Locals)
        locals.push(parseBitmap(bm));
    for (const bm of info.Liveness.Args)
        args.push(parseBitmap(bm));

    // Compute varp/argp and covered range.
    var liveMin = 0xffffffff;
    var liveMax = 0;
    var argMin = 0xffffffff;
    var argMax = 0;
    const insts = [];
    for (let i = 0; i < info.Insts.length; i++) {
        const spoff = info.Liveness.SPOff[i];
        insts[i] = {
            varp: info.Liveness.VarpDelta + spoff,
            argp: info.Liveness.ArgpDelta + spoff,
        };

        // SPOff -1 indicates unknown SP offset. SPOff 0 happens at
        // RET, where we don't bother resetting the liveness index,
        // but if the frame is 0 bytes, it can't have liveness.
        if (spoff <= 0)
            continue;

        const index = info.Liveness.Indexes[i];
        if (index < 0)
            continue;
        if (insts[i].varp > 0) {
            insts[i].localp = insts[i].varp - locals[index].n * ptrSize;
            liveMin = Math.min(liveMin, insts[i].localp);
            liveMax = Math.max(liveMax, insts[i].varp);
        }
        if (insts[i].argp > 0) {
            argMin = Math.min(argMin, insts[i].argp);
            argMax = Math.max(argMax, insts[i].argp + args[index].n * ptrSize);
        }
    }
    const haveArgs = argMin < argMax;

    // Create table header.
    if (liveMin < liveMax)
        table.groupHeader.append($("<th>").text("locals").addClass("flag").attr("colspan", (liveMax - liveMin) / ptrSize));
    if (haveArgs) {
        table.groupHeader.append($("<th>"));
        table.groupHeader.append($("<th>").text("args").addClass("flag").attr("colspan", (argMax - argMin) / ptrSize));
    }

    for (let i = liveMin; i < liveMax; i += ptrSize)
        table.header.append($("<th>").text("0x"+i.toString(16)).addClass("flag"));
    if (haveArgs) {
        table.header.append($("<th>").text("|").addClass("flag"));
        for (let i = argMin; i < argMax; i += ptrSize)
            table.header.append($("<th>").text("0x"+i.toString(16)).addClass("flag"));
    }

    // Create table cells.
    for (var i = 0; i < rows.length; i++) {
        const row = rows[i];
        const index = info.Liveness.Indexes[i];
        const addBit = function(addr, bitmap, base) {
            const i = (addr - base) / ptrSize;
            var text = "";
            if (bitmap !== undefined && 0 <= i && i < bitmap.n)
                text = bitmap.bit(i) ? "P" : "-";
            row.elt.append($("<td>").text(text).addClass("flag"));
        };
        for (let addr = liveMin; addr < liveMax; addr += ptrSize)
            addBit(addr, locals[index], insts[i].localp);
        if (haveArgs) {
            row.elt.append($("<td>"));
            for (let addr = argMin; addr < argMax; addr += ptrSize)
                addBit(addr, args[index], insts[i].argp);
        }
    }
}

function Bitmap(nbits, bytes) {
    this.n = nbits;
    this.bytes = bytes;
}

Bitmap.prototype.bit = function(n) {
    if (n < 0 || n >= this.nbits)
        throw "out of range";
    return (this.bytes[Math.floor(n/8)]>>(n%8)) & 1;
};

function parseBitmap(bitmap) {
    // Parse hex representation.
    const parts = bitmap.split(":", 2);
    const nbits = parseInt(parts[0], 10);
    const bytes = [];
    for (var i = 0; i < parts[1].length; i += 2) {
        bytes.push(parseInt(parts[1].substr(i, 2), 16));
    }
    return new Bitmap(nbits, bytes);
}
