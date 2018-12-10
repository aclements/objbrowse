// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

class SourceView {
    constructor(data, container) {
        const table = $("<table>").css({borderCollapse: "collapse"}).appendTo(container);
        this._table = table;

        let prevPath = "";
        let pcRanges = [];
        for (let block of data.Blocks) {
            const th = $('<th colspan="2">');
            if (block.Path == prevPath) {
                th.text("\u22ef");
            } else {
                th.addClass('sv-path').text(block.Path);
                prevPath = block.Path;
            }
            table.append($('<tr>').append(th));

            if (block.Error) {
                const th = $('<th colspan="2">').addClass('sv-error').text(block.Error);
                table.append($('<tr>').append(th));
            }

            let lineNo = block.Start;
            for (let i = 0; i < block.Text.length; i++) {
                const tr = $('<tr>').append(
                    $('<td>').addClass('sv-line').text(lineNo)
                ).append(
                    $('<td>').addClass('sv-src').text(block.Text[i])
                );
                table.append(tr);
                let pcs = block.PCs[i];
                if (pcs) {
                    tr.click(() => { highlightRanges(pcs); });
                    for (let r of pcs) {
                        pcRanges.push([r[0], r[1], tr]);
                    }
                } else {
                    tr.click(() => { highlightRanges([]); });
                }
                lineNo++;
            }
        }

        this._pcRanges = new IntervalMap(pcRanges);
    }

    highlightRanges(ranges) {
        // Clear highlights.
        $(".highlight", this._table).removeClass("highlight");

        // New highlights.
        for (let match of this._pcRanges.intersect(ranges)) {
            match[2].addClass("highlight");
        }
    }
}
