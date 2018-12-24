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
