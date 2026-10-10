/**
 * 通用的方法
 *
 * @author xiaoRui
 */
export default {
    // 展开树形数据方法
    handleTree(data, id, parentId, children) {
        let config = {
            id: id || 'id',
            parentId: parentId || 'parentId',
            childrenList: children || 'children'
        };
        var childrenListMap = {};
        var nodeIds = {};
        var tree = [];
        for (let d of data) {
            let parentId = d[config.parentId];
            let nodeId = d[config.id];
            nodeIds[nodeId] = d;

            // Some APIs represent the root with parentId equal to its own id.
            // Do not add that self-reference as a child, otherwise tree building
            // would recurse forever when adapting children.
            if (String(parentId) !== String(nodeId)) {
                if (childrenListMap[parentId] == null) {
                    childrenListMap[parentId] = [];
                }
                childrenListMap[parentId].push(d);
            }
        }
        for (let d of data) {
            let parentId = d[config.parentId];
            // Treat self-parented nodes as roots as well as nodes whose parent
            // is absent from the response.
            if (nodeIds[parentId] == null || String(parentId) === String(d[config.id])) {
                tree.push(d);
            }
        }
        for (let t of tree) {
            adaptToChildrenList(t);
        }
        function adaptToChildrenList(o) {
            if (childrenListMap[o[config.id]] !== null) {
                o[config.childrenList] = childrenListMap[o[config.id]];
            }
            if (o[config.childrenList]) {
                for (let c of o[config.childrenList]) {
                    adaptToChildrenList(c);
                }
            }
        }
        return tree;
    }
}
