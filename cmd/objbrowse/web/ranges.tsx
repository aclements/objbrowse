/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

type Range = { start: number, end: number }

/**
 * Ranges is a set of intervals.
 */
export class Ranges {
    private ranges: Range[];

    // ranges must be an array of objects with "start" and "end"
    // properties, where each object covers a disjoint range [start,
    // end).
    constructor(ranges?: Range[]) {
        if (ranges === undefined) {
            this.ranges = [];
            return;
        }
        // Sort ranges.
        ranges.sort((a, b) => a.start - b.start);
        this.ranges = ranges;
    }
}
