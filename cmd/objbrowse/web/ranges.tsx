/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

export type Range = { start: bigint, end: bigint }

/**
 * Ranges is a set of intervals.
 */
export class Ranges {
    ranges: Range[];

    // ranges must be an array of objects with "start" and "end"
    // properties, where each object covers a disjoint range [start,
    // end).
    constructor(ranges?: Range[]) {
        if (ranges === undefined) {
            this.ranges = [];
            return;
        }
        // Sort ranges.
        ranges.sort((a, b) => a < b ? -1 : a > b ? 1 : 0);
        this.ranges = ranges;
    }

    /**
     * index returns the index of the first range that ends after val,
     * or ranges.length if none do. It may or may not contain val.
     */
    private index(val: bigint) {
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

    /**
     * find returns the range containing point, or null.
     */
    find(point: bigint): Range | null {
        const i = this.index(point);
        if (i < this.ranges.length && contains(this.ranges[i], point)) {
            return this.ranges[i];
        }
        return null;
    }

    /**
     * anyIntersection returns whether the intersection of this and
     * ranges is empty.
     */
    anyIntersection(ranges: Range | Ranges): boolean {
        if (this.ranges.length === 0) {
            return false;
        }
        for (let range of rangeList(ranges)) {
            const i = this.index(range.start);
            if (i < this.ranges.length && overlaps(range, this.ranges[i])) {
                return true;
            }
        }
        return false;
    }
}

function rangeList(ranges: Range | Ranges): Range[] {
    if (ranges instanceof Ranges) {
        return ranges.ranges;
    }
    return [ranges];
}

function overlaps(r1: Range, r2: Range): boolean {
    return r1.end > r2.start && r1.start < r2.end;
}

function contains(r: Range, point: bigint): boolean {
    return r.start <= point && point < r.end;
}
