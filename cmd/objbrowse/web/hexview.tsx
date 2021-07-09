/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React from "react";

import { ViewProps } from "./objbrowse";
import { useFetchJSON } from "./hooks";

import "./hexview.css";

type json = { Addr: string, Data: string }

function HexViewer(props: ViewProps) {
    const fetch = useFetchJSON(`/sym/${props.entity.id}/hex`)
    if (fetch.pending) {
        return fetch.pending;
    }
    if (typeof fetch.value === "string") {
        // "No data" Error
        return <div>{fetch.value}</div>;
    }
    const v: json = fetch.value;

    const start = BigInt("0x" + v.Addr);
    const len = v.Data.length / 2;

    // Format column header.
    let hexHead = "", asciiHead = "";
    let headStart = Number(start % BigInt(16));
    for (let i = 0; i < 16; i++) {
        if (i == 8) {
            hexHead += "  ";
        } else if (i > 0) {
            hexHead += " ";
        }
        const digit = ((headStart + i) % 16).toString(16)
        hexHead += " " + digit;
        asciiHead += digit;
    }

    // TODO: Maybe a line should always start at a 0 offset and just
    // insert spaces to get to the first byte. The rendering of things
    // that don't start 16-byte aligned is really kind of weird. (Look
    // at, e.g., runtime.call*.args_stackmap.)

    // TODO: Support for selecting ranges, both in the hex or the ASCII.
    // Measure text dimensions by creating a hidden div with a
    // placeholder hex line with spans around the elements. From that we
    // can map the X coordinate (and if we move away from table rows,
    // use the height to map the Y coordinate). Or maybe I just register
    // a mouseUp handler and adjust the window.getSelection at that
    // point (check that that works with dragging, double clicking, and
    // shift-clicking).
    //
    // Ideally we would build our selection on the DOM Selection API to
    // stay consistent with platform selection rules, but modifying the
    // selection while selection is in progress aborts the selection
    // drag, and there doesn't appear to be a way to detect that the
    // user is done making a selection and then modify it. So we
    // implement our own mouse handling.

    // TODO: Jump to address.

    // TODO: Doing this with long divs would create far fewer DOM elements. If I
    // layout each column in its own inline-block, selection would also be more
    // natural (though maybe I have to totally override that anyway). OTOH, if
    // I'm going to the trouble of making this lazy, a table may just be easier
    // and the cost irrelevant.

    const rows: React.ReactElement[] = [];
    for (let off = 0; off < len; off += 16) {
        const [hex, ascii] = formatLine(v.Data, off);
        rows.push(
            <tr key={off}>
                <td>0x{(start + BigInt(off)).toString(16)}</td>
                <td>{hex}</td>
                <td>{ascii}</td>
            </tr>
        )
    }

    const addrWidth = 2 + (start + BigInt(len)).toString(16).length;
    return (<table className="hv-table" >
        <colgroup>
            <col style={{ width: addrWidth + "ch" }}></col>
            <col style={{ width: (3 * 16) + "ch" }}></col>
            <col style={{ width: "16ch" }}></col>
        </colgroup>
        <thead><tr><th></th><th>{hexHead}</th><th>{asciiHead}</th></tr></thead>
        <tbody>{rows}</tbody>
    </table>);
}

function formatLine(data: string, start: number) {
    const dataLen = data.length / 2;
    let line = "";
    let ascii = "";
    for (let i = 0; i < 16 && start + i < dataLen; i++) {
        if (i == 8)
            line += "  ";
        else if (i > 0)
            line += " ";
        const hex = data.substr((start + i) * 2, 2);
        line += hex;
        const val = parseInt(hex, 16);
        if (32 <= val && val <= 126)
            ascii += String.fromCharCode(val);
        else
            ascii += ".";
    }
    return [line, ascii];
}

export const HexView = { element: HexViewer, name: "hex" };
