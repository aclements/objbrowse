/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// TODO: Implement selection syncing

import React from "react";

import { ViewProps } from "./objbrowse";
import { useFetchJSON } from "./hooks";

import "./sourceview.css";

type json = { Blocks: block[] }
type block = { Path: string, Func: string, Start: number, Text: string[], PCs: string[][][], Error?: string }

function SourceViewer(props: ViewProps) {
    // Fetch data.
    const fetch = useFetchJSON(`/sym/${props.value.entity.id}/source`)
    if (fetch.pending) {
        return fetch.pending;
    }
    const v: json = fetch.value;

    let blocks = v.Blocks.map((val, blockI) => {
        return <Block key={blockI} block={val}></Block>;
    });

    return <>{blocks}</>;
}

function Block(props: { block: block }) {
    const b = props.block;

    let slash = b.Path.lastIndexOf("/") + 1;
    let info = (<tr className="sv-path">
        <td colSpan={2}>
            {b.Func &&
                <span className="fw-bold">{b.Func + " "}</span>}
            <span className="text-muted">{b.Path.substr(0, slash)}</span>{b.Path.substr(slash)}
        </td>
    </tr>);

    let rows;
    if (b.Error) {
        rows = <tr><td>{b.Error}</td></tr>;
    } else {
        rows = b.Text.map((text, line) => {
            let pcs = b.PCs[line];
            let cls = "sv-src";
            if (pcs.length === 0) {
                cls += " text-muted"
            }
            return (<tr key={line}>
                <td className="sv-line">{b.Start + line}</td>
                <td className={cls}>{text}</td>
            </tr>);
        });
    }

    return (<table className="sv-table">
        <thead>
            {info}
        </thead>
        <tbody>
            {rows}
        </tbody>
    </table>);
}

export const SourceView = { element: SourceViewer, id: "source", label: "Source" };
