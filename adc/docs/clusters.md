# Cluster Graph

This figure places sampled juror responses in a two-dimensional projection.  Each point represents one sampled completion for one model, one persona file, and one gene prompt.  Rows separate model sources, columns separate gene prompts, and `X1` and `X2` are the plotted coordinates.

| Element | Meaning |
|---|---|
| Point | One sampled completion |
| Row | One model source, such as `anthropic`, `google`, or `openai` |
| Column | One gene prompt |
| Color | Cluster label within that gene |
| Shape | Full model name within that source row |

Within a gene column, color marks cluster assignment.  Because clustering is gene-specific, the same color in another column refers to a different partition.  Within a source row, shape marks the full model name, and the legend at the right of that row identifies those models.  Marker shapes recur in other rows, where they refer to different models from different sources.

Each panel shows how models from one source distribute on one gene.  Nearby points indicate responses that lie close in the projected space.  A dense group of one color indicates one cluster.  Several shapes inside that group show different models from the same source landing there.  One shape spread across several colors shows one model producing several response patterns across repeated samples.

Panel position is local.  Each gene has its own projected space, and each panel scales to the data it contains.  The figure records grouping and separation in that space rather than model quality, legal merit, or preference.
