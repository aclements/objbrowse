// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

class SymView {
    constructor(data, container) {
        const self = this;
        this._allSyms = data.Syms;
        this._sort = "name";
        $(container).addClass("symview");

        // Parse symbol addresses.
        for (let sym of data.Syms) {
            sym[2] = new AddrJS(sym[2]);
        }

        // Add search box.
        //
        // TODO: Also accept an address to search for.
        const search = $('<input type="text" size="40" autofocus="true" placeholder="filter regexp">').appendTo(container);
        let searchDelay = null;
        self._filterRe = null;
        function onSearch(now) {
            if (search.val() == "") {
                self._filterRe = null;
            } else {
                try {
                    self._filterRe = new RegExp(search.val());
                } catch (error) {
                    search[0].setCustomValidity(error.message);
                    return;
                }
            }
            search[0].setCustomValidity("");

            // This can be expensive to apply, so wait for a bit of
            // idle time before refreshing.
            if (searchDelay !== null) {
                clearTimeout(searchDelay);
                searchDelay = null;
            }
            if (now) {
                self._updateFilter();
            } else {
                searchDelay = setTimeout(self._updateFilter.bind(self), 50);
            }
        }
        search.on('input', () => { onSearch(false); });
        // setCustomValidity above will change the input color as the
        // user types. If they try to accept an invalid regexp, have
        // the browser show the validation message.
        search.change(() => { onSearch(true); search[0].reportValidity(); });

        // Add table.
        const table = $('<table class="symview-table">').appendTo(container);
        this._table = table;

        // The browser may populate the input form from the history
        // (for some reason this can take a moment and doesn't trigger
        // input events), so parse the filter and populate the table.
        setTimeout(() => {
            onSearch(true);
        }, 1);
    }

    _updateFilter() {
        // Create a filtered copy of the syms list.
        if (this._filterRe == null) {
            this._syms = this._allSyms;
            this._populate();
            return;
        }

        const syms = [];
        for (let sym of this._allSyms) {
            if (this._filterRe.test(sym[0])) {
                syms.push(sym);
            }
        }
        this._syms = syms;

        this._populate();
    }

    _populate() {
        const self = this;

        // Sym array indexes.
        const NAME = 0;
        const TYPE = 1;
        const VALUE = 2;

        // Crete table header.
        const t = this._table;
        t.empty();
        const colName = $('<td width="30em">Name</td>');
        const colType = $('<td width="3em">Type</td>');
        const colValue = $('<td width="10em">Value</td>');
        t.css({"width": (30+3+10)+"em"});
        t.append(
            $('<thead>').append(colName).append(colType).append(colValue)
        );
        colName.click(() => { self._sort = "name"; self._populate(); });
        colValue.click(() => { self._sort = "value"; self._populate(); });
        $([colName[0], colValue[0]]).css({"cursor": "pointer"});

        // Sort symbols.
        const syms = this._syms;
        let sortCol;
        if (this._sort == "name") {
            syms.sort((a, b) => a[NAME] < b[NAME] ? -1 : +(a[NAME] > b[NAME]));
            sortCol = colName;
        } else if (this._sort == "value") {
            syms.sort((a, b) => a[VALUE].compare(b[VALUE]));
            sortCol = colValue;
        } else {
            throw("bad sort " + this._sort);
        }
        sortCol.text(sortCol.text() + " \u2193").css({"font-weight": "bold"});

        // Populate table lazily.
        const blockLines = 1000
        new LazyTable(t, syms.length, blockLines, (start, n) => {
            const rows = [];
            for (let i = start; i < start + n; i++) {
                const sym = self._syms[i];
                const tr = $('<tr>').append([
                    $('<td>').addClass('symview-name').text(sym[NAME]),
                    $('<td>').text(sym[TYPE]),
                    $('<td>').text(sym[VALUE]),
                ]);
                tr.click(() => { window.location.href = '/s/' + sym[NAME]; })
                rows.push(tr[0]);
            }
            return rows;
        });
    }
}
