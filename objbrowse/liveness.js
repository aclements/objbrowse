// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

class LivenessOverlay {
    constructor(info) {
        const ptrSize = info.PtrSize;
        this._ptrSize = ptrSize;

        // Decode live bitmaps.
        this._locals = [];
        this._args = [];
        for (const bm of info.Locals)
            this._locals.push(parseBitmap(bm));
        for (const bm of info.Args)
            this._args.push(parseBitmap(bm));

        // Create interval maps for SP offset and bitmap indexes.
        function mkMap(json) {
            for (let r of json) {
                r.start = new AddrJS(r.start);
                r.end = new AddrJS(r.end);
            }
            return new IntervalMap(json);
        }
        const spOff = mkMap(info.SPOff);
        const indexes = mkMap(info.Indexes);

        // Join the two maps to compute varp/argp/localp.
        let liveMin = 0xffffffff;
        let liveMax = 0;
        let argMin = 0xffffffff;
        let argMax = 0;
        const lmap = spOff.intersectJoin(indexes, function(sp, index, out) {
            out.spOff = sp.val;
            out.index = index.val;
            out.varp = info.VarpDelta + sp.val;
            out.argp = info.ArgpDelta + sp.val;

            if (out.varp > 0) {
                out.localp = out.varp - this._locals[index.val].n * ptrSize;
                liveMin = Math.min(liveMin, out.localp);
                liveMax = Math.max(liveMax, out.varp);
            }
            if (out.argp > 0) {
                argMin = Math.min(argMin, out.argp);
                argMax = Math.max(argMax, out.argp + this._args[index.val].n * ptrSize);
            }
        }.bind(this));
        this._liveMin = liveMin;
        this._liveMax = liveMax;
        this._argMin = argMin;
        this._argMax = argMax;
        this._lmap = lmap;
        this._haveArgs = argMin < argMax;
    }

    // render adds liveness data to a table. rowMap is an IntervalMap
    // from addresses to rows, where each value has a "tr" property
    // that is a DOM "tr" element. "table" is an object with "header"
    // and "groupHeader" properties.
    render(table, rowMap) {
        // TODO: Make this lazy, as it can be quite expensive even for
        // reasonably sized functions. Maybe I should generalize LazyTable
        // so that rather than having a fixed block size, it renders
        // enough rows to fill the screen, plus as many more as it can do
        // under some time bound. Alternatively, I could fill in the whole
        // table, but allow browser refreshes regularly during the
        // process.

        const ptrSize = this._ptrSize;
        const liveMin = this._liveMin, liveMax = this._liveMax;
        const argMin = this._argMin, argMax = this._argMax;

        // Create table header.
        if (liveMin < liveMax)
            $(table.groupHeader).append($("<th>").text("locals").addClass("flag").attr("colspan", (liveMax - liveMin) / ptrSize));
        if (this._haveArgs) {
            $(table.groupHeader).append($("<th>"));
            $(table.groupHeader).append($("<th>").text("args").addClass("flag").attr("colspan", (argMax - argMin) / ptrSize));
        }

        for (let i = liveMin; i < liveMax; i += ptrSize)
            $(table.header).append($("<th>").text("0x"+i.toString(16)).addClass("flag"));
        if (this._haveArgs) {
            $(table.header).append($("<th>").text("|").addClass("flag"));
            for (let i = argMin; i < argMax; i += ptrSize)
                $(table.header).append($("<th>").text("0x"+i.toString(16)).addClass("flag"));
        }

        // Create table cells.
        for (let r of rowMap.ranges) {
            // Get liveness data that intersects with this row.
            const ls = this._lmap.intersect([r]);
            const tr = $(r.tr);
            const addBit = function(addr, bitmapSet, spProp) {
                // Join together the bits from each liveness map at this row.
                let text = "";
                for (let l of ls) {
                    const bitmap = bitmapSet[l.index];
                    const bmi = (addr - l[spProp]) / ptrSize;
                    let thisText;
                    if (bmi < 0 || bmi >= bitmap.n)
                        thisText = "-";
                    else
                        thisText = bitmap.bit(bmi) ? "P" : "-";
                    if (text == "")
                        text = thisText;
                    else if (text != thisText)
                        text = "?";
                }
                // Add table cell.
                tr.append($("<td>").text(text).addClass("flag"));
            };
            for (let addr = liveMin; addr < liveMax; addr += ptrSize)
                addBit(addr, this._locals, "localp");
            if (this._haveArgs) {
                tr.append($("<td>"));
                for (let addr = argMin; addr < argMax; addr += ptrSize)
                    addBit(addr, this._args, "argp");
            }
        }
    }
}

class Bitmap {
    constructor(nbits, bytes) {
        this.n = nbits;
        this.bytes = bytes;
    }

    bit(n) {
        if (n < 0 || n >= this.nbits)
            throw "out of range";
        return (this.bytes[Math.floor(n/8)]>>(n%8)) & 1;
    }
}

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
