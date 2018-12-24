// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

// IntervalMap is a map of intervals over AddrJS values.
class IntervalMap {
    // ranges must be an array of objects with "start" and "end"
    // properties, where each object covers a disjoint range [start,
    // end).
    constructor(ranges) {
        // Sort ranges.
        ranges.sort((a, b) => a.start.compare(b.start));
        this.ranges = ranges;
    }

    // _find returns the index of the first range that ends after val.
    // It may or may not contain val.
    _find(val) {
        let lo = 0, hi = this.ranges.length;
        while (lo < hi) {
            let mid = Math.floor((lo + hi) / 2);
            if (this.ranges[mid].end > val)
                hi = mid;
            else
                lo = mid + 1;
        }
        return lo;
    }

    // get returns the range object containing val, if any, or null.
    get(val) {
        let i = this._find(val);
        if (i < this.ranges.length && this.ranges[i].start <= val)
            return this.ranges[i];
        return null;
    }

    // Intersect a list of ranges with the ranges in this map and
    // return the subset that overlap. ranges must already be sorted.
    intersect(ranges) {
        const out = [];
        if (ranges.length == 0)
            return out;
        const end = ranges[ranges.length-1].end;
        let i = this._find(ranges[0].start), j = 0;
        while (i < this.ranges.length && j < ranges.length) {
            if (this.ranges[i].start >= end)
                break;
            if (IntervalMap.overlap(this.ranges[i], ranges[j])) {
                // Found a match.
                out.push(this.ranges[i]);
                i++;
            } else if (this.ranges[i].start < ranges[j].start) {
                i++;
            } else {
                j++;
            }
        }
        return out;
    }

    // intersectJoin calls fn(r1, r2, rout) for each range r1 that
    // appears in this IntervalMap and r2 that appears in IntervalMap
    // b, where r1 and r2 overlap. fn may add fields to rout, which is
    // initially the range that overlaps between r1 and r2.
    // intersectJoin returns the IntervalMap consisting of rout
    // values.
    intersectJoin(b, fn) {
        const out = [];
        let i = 0, j = 0;
        while (i < this.ranges.length && j < b.ranges.length) {
            const r1 = this.ranges[i], r2 = b.ranges[j];
            const rout = IntervalMap.intersection(r1, r2);
            if (rout !== null) {
                // Found an intersection.
                fn(r1, r2, rout);
                out.push(rout);
                if (r1.end == rout.end)
                    i++;
                if (r2.end == rout.end)
                    j++;
            } else if (this.ranges[i].start < b.ranges[j].start) {
                i++;
            } else {
                j++;
            }
        }
        return new IntervalMap(out);
    }

    static overlap(r1, r2) {
        return r1.end.compare(r2.start) > 0 && r1.start.compare(r2.end) < 0;
    }

    static intersection(r1, r2) {
        let start = r1.start;
        if (start.compare(r2.start) < 0)
            start = r2.start;
        let end = r1.end;
        if (end.compare(r2.end) > 0)
            end = r2.end;
        if (start.compare(end) < 0)
            return {start: start, end: end};
        return null;
    }
}

// AddrJS works in 28 bit digits because Javascript treats numbers as
// signed 32 bit values and we need something that divides evenly into
// hex digits.
const AddrJSBits = 28;
const AddrJSDigits = AddrJSBits / 4;

// AddrJS is a positive arbitrary precision integer, used for
// representing memory addresses.
class AddrJS {
    // AddrJS optionally takes a hex string or a number.
    //
    // TODO: Support negative numbers.
    constructor(x, _digits) {
        if (_digits !== undefined) {
            this._digits = _digits;
            this._trim();
            return;
        }
        if (x === undefined || x == 0) {
            this._digits = [];
            return;
        }

        let digits = [];
        const typ = Object.prototype.toString.call(x);
        switch (typ) {
        case "[object String]":
            for (let i = 0; i < x.length; i += AddrJSDigits) {
                let end = x.length - i;
                let start = end - AddrJSDigits;
                if (start < 0) start = 0;
                const part = x.substring(start, end);
                digits.push(parseInt(part, 16));
            }
            break;
        case "[object Number]":
            const mask = (1<<AddrJSBits) - 1;
            while (x > 0) {
                digits.push(x & mask);
                x = x >> AddrJSBits;
            }
            break
        default:
            throw "not a string or number";
        }
        this._digits = digits;
    }

    _trim() {
        const d = this._digits;
        while (d.length > 0 && d[d.length-1] == 0)
            d.pop();
    }

    toString() {
        const d = this._digits;
        if (d.length == 0)
            return "0";
        let i = d.length - 1;
        let out = d[i].toString(16);
        for (i--; i >= 0; i--)
            out += d[i].toString(16).padStart(AddrJSDigits, "0");
        return out;
    }

    toNumber() {
        let n = 0, shift = 0;
        for (let digit of this._digits) {
            n += digit << shift
            shift += AddrJSBits;
        }
        return n;
    }

    // compare returns -1 if this < b, 0 if this == b, and 1 if this > b.
    compare(b) {
        const x = this._digits, y = b._digits;
        if (x.length < y.length)
            return -1;
        if (x.length > y.length)
            return 1;
        for (let i = x.length - 1; i >= 0; i--) {
            if (x[i] < y[i])
                return -1;
            if (x[i] > y[i])
                return 1;
        }
        return 0;
    }

    // add return this + b as a new AddrJS value.
    add(b) {
        const x = this._digits, y = b._digits;
        const digits = Math.max(x.length, y.length);

        let carry = 0;
        let out = [];
        for (let i = 0; i < digits; i++) {
            let o = (x[i] || 0) + (y[i] || 0) + carry;
            carry = o >> AddrJSBits;
            o = o & ((1 << AddrJSBits) - 1);
            out.push(o);
        }
        out.push(carry);

        return new AddrJS(undefined, out);
    }

    // sub return this - b as a new AddrJS value.
    sub(b) {
        const x = this._digits, y = b._digits;
        const digits = Math.max(x.length, y.length);

        let borrow = 0;
        let out = [];
        for (let i = 0; i < digits; i++) {
            let xx = x[i] || 0;
            let yy = (y[i] || 0) + borrow;
            borrow = (xx < yy) + 0;
            xx += borrow<<AddrJSBits;
            out.push(xx - yy);
        }
        if (borrow)
            throw("negative result");
        return new AddrJS(undefined, out);
    }
}

class Panels {
    constructor(container) {
        this._c = $("<div>").css({position: "relative", height: "100%", display: "flex"});
        this._c.appendTo(container);
        this._cols = [];
    }

    // addCol adds a column to the right and returns a container
    // element.
    addCol() {
        const div = $("<div>").css({
            overflow: "auto", height: "100%", boxSizing: "border-box",
            padding: "8px", flex: "1",
        });
        if (this._cols.length >= 0) {
            div.css({borderLeft: "2px solid #888"});
        }
        this._c.append(div);
        this._cols.push({div: div});
        return div[0];
    }
}

function scrollTo(container, elt) {
    $(container).animate({
        scrollTop: $(elt).position().top
    }, 500);
}

var asmView;
var sourceView;
var hexView;
var baseAddr;

function render(container, info) {
    const panels = new Panels(container);
    if (info.HexView)
        hexView = new HexView(info.HexView, panels.addCol());
    if (info.AsmView)
        asmView = new AsmView(info.AsmView, panels.addCol());
    if (info.SourceView)
        sourceView = new SourceView(info.SourceView, panels.addCol());

    baseAddr = new AddrJS(info.Base);

    window.addEventListener("hashchange", onHashChange, false);
    $.fx.off = true;  // Inhibit scrolling animations during setup.
    onHashChange();
    $.fx.off = false;
}

function onHashChange() {
    let hash = window.location.hash;
    if (onHashChange.lastHash === hash)
        return;
    onHashChange.lastHash = hash;
    if (hash != "" && hash != "#") {
        hash = hash.substr(1);  // Trim "#"
        const relative = hash[0] == "+";
        if (relative)
            hash = hash.substr(1);
        const ranges = parseRanges(hash);
        if (relative) {
            for (let r of ranges) {
                r.start = r.start.add(baseAddr);
                r.end = r.end.add(baseAddr);
            }
        }
        highlightRanges(ranges, null);
    }
}

function highlightRanges(ranges, cause) {
    if (hexView)
        hexView.highlightRanges(ranges, cause !== hexView);
    if (asmView)
        asmView.highlightRanges(ranges, cause !== asmView);
    if (sourceView)
        sourceView.highlightRanges(ranges, cause !== sourceView);

    const newHash = "#" + formatRanges(ranges);
    onHashChange.lastHash = newHash; // Inhibit hashchange listener
    window.location.hash = newHash
}

function formatRanges(ranges) {
    let out = "";
    for (let r of ranges) {
        if (out.length > 0)
            out += ","
        out += r.start.toString() + "-" + r.end.toString();
    }
    return out;
}

function parseRanges(ranges) {
    const out = [];
    for (let r of ranges.split(",")) {
        const parts = r.split("-", 2);
        out.push({start: new AddrJS(parts[0]), end: new AddrJS(parts[1])});
    }
    return out;
}

// renderLiveness adds liveness data from info. rowMap is an
// IntervalMap from addresses to rows, where each value has a "tr"
// property that is a DOM "tr" element. "table" is an object with
// "header" and "groupHeader" properties.
function renderLiveness(info, rowMap, table) {
    // TODO: Make this lazy, as it can be quite expensive even for
    // reasonably sized functions. Maybe I should generalize LazyTable
    // so that rather than having a fixed block size, it renders
    // enough rows to fill the screen, plus as many more as it can do
    // under some time bound. Alternatively, I could fill in the whole
    // table, but allow browser refreshes regularly during the
    // process.

    const ptrSize = info.PtrSize;

    // Decode live bitmaps.
    const locals = [], args = [];
    for (const bm of info.Locals)
        locals.push(parseBitmap(bm));
    for (const bm of info.Args)
        args.push(parseBitmap(bm));

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
    var liveMin = 0xffffffff;
    var liveMax = 0;
    var argMin = 0xffffffff;
    var argMax = 0;
    const lmap = spOff.intersectJoin(indexes, function(sp, index, out) {
        out.spOff = sp.val;
        out.index = index.val;
        out.varp = info.VarpDelta + sp.val;
        out.argp = info.ArgpDelta + sp.val;

        if (out.varp > 0) {
            out.localp = out.varp - locals[index.val].n * ptrSize;
            liveMin = Math.min(liveMin, out.localp);
            liveMax = Math.max(liveMax, out.varp);
        }
        if (out.argp > 0) {
            argMin = Math.min(argMin, out.argp);
            argMax = Math.max(argMax, out.argp + args[index.val].n * ptrSize);
        }
    });
    const haveArgs = argMin < argMax;

    // Create table header.
    if (liveMin < liveMax)
        $(table.groupHeader).append($("<th>").text("locals").addClass("flag").attr("colspan", (liveMax - liveMin) / ptrSize));
    if (haveArgs) {
        $(table.groupHeader).append($("<th>"));
        $(table.groupHeader).append($("<th>").text("args").addClass("flag").attr("colspan", (argMax - argMin) / ptrSize));
    }

    for (let i = liveMin; i < liveMax; i += ptrSize)
        $(table.header).append($("<th>").text("0x"+i.toString(16)).addClass("flag"));
    if (haveArgs) {
        $(table.header).append($("<th>").text("|").addClass("flag"));
        for (let i = argMin; i < argMax; i += ptrSize)
            $(table.header).append($("<th>").text("0x"+i.toString(16)).addClass("flag"));
    }

    // Create table cells.
    for (let r of rowMap.ranges) {
        const ls = lmap.intersect([r]);
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
            addBit(addr, locals, "localp");
        if (haveArgs) {
            tr.append($("<td>"));
            for (let addr = argMin; addr < argMax; addr += ptrSize)
                addBit(addr, args, "argp");
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
