import type { NodeTagApiItem, NodeTagMutationPayload } from "@/api/types";

import { useEffect, useState } from "react";
import toast from "react-hot-toast";

import {
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
} from "@/shadcn-bridge/heroui/modal";
import { Button } from "@/shadcn-bridge/heroui/button";
import { Input } from "@/shadcn-bridge/heroui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
} from "@/shadcn-bridge/heroui/table";
import {
  createNodeTag,
  deleteNodeTag,
  getNodeTagList,
  updateNodeTag,
} from "@/api";

interface NodeTagManagerProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onTagChange?: () => void;
}

export function NodeTagManager({
  isOpen,
  onOpenChange,
  onTagChange,
}: NodeTagManagerProps) {
  const [tags, setTags] = useState<NodeTagApiItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [editingTag, setEditingTag] = useState<NodeTagApiItem | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  const loadTags = async () => {
    setLoading(true);
    try {
      const res = await getNodeTagList();

      setTags(res.data || []);
    } catch {
      toast.error("加载标签列表失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      void loadTags();
    }
  }, [isOpen]);

  const handleOpenModal = (tag?: NodeTagApiItem) => {
    setEditingTag(tag || null);
    setIsModalOpen(true);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
    setEditingTag(null);
  };

  const handleSave = async (data: NodeTagMutationPayload) => {
    try {
      if (editingTag) {
        await updateNodeTag({ ...data, id: editingTag.id });
        toast.success("标签更新成功");
      } else {
        await createNodeTag(data);
        toast.success("标签创建成功");
      }

      handleCloseModal();
      await loadTags();
      onTagChange?.();
    } catch {
      toast.error(editingTag ? "更新标签失败" : "创建标签失败");
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("确定要删除此标签吗？相关节点的标签关联也会一并移除。")) {
      return;
    }

    try {
      await deleteNodeTag(id);
      toast.success("标签删除成功");
      await loadTags();
      onTagChange?.();
    } catch {
      toast.error("删除标签失败");
    }
  };

  return (
    <>
      <Modal
        backdrop="blur"
        classNames={{
          base: "!w-[calc(100%-32px)] !mx-auto sm:!w-full rounded-2xl overflow-hidden",
        }}
        isOpen={isOpen}
        placement="center"
        size="2xl"
        onOpenChange={onOpenChange}
      >
        <ModalContent>
          <ModalHeader>节点标签管理</ModalHeader>
          <ModalBody>
            <div className="mb-4">
              <Button color="primary" onPress={() => handleOpenModal()}>
                创建标签
              </Button>
            </div>

            {loading ? (
              <div className="flex justify-center py-8">
                <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
              </div>
            ) : tags.length === 0 ? (
              <div className="py-8 text-center text-gray-500">暂无标签</div>
            ) : (
              <Table aria-label="节点标签列表">
                <TableHeader>
                  <TableColumn>名称</TableColumn>
                  <TableColumn>颜色</TableColumn>
                  <TableColumn>节点数</TableColumn>
                  <TableColumn>操作</TableColumn>
                </TableHeader>
                <TableBody>
                  {tags.map((tag) => (
                    <TableRow key={tag.id}>
                      <TableCell>
                        <div className="font-medium">{tag.name}</div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <div
                            className="h-4 w-4 rounded"
                            style={{ backgroundColor: tag.color }}
                          />
                          <span className="text-sm text-gray-600">
                            {tag.color}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>{tag.nodeCount}</TableCell>
                      <TableCell>
                        <div className="flex gap-2">
                          <Button
                            size="sm"
                            variant="flat"
                            onPress={() => handleOpenModal(tag)}
                          >
                            编辑
                          </Button>
                          <Button
                            color="danger"
                            size="sm"
                            variant="flat"
                            onPress={() => handleDelete(tag.id)}
                          >
                            删除
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={() => onOpenChange(false)}>
              关闭
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <TagEditModal
        isOpen={isModalOpen}
        tag={editingTag}
        onOpenChange={setIsModalOpen}
        onSave={handleSave}
      />
    </>
  );
}

interface TagEditModalProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  tag: NodeTagApiItem | null;
  onSave: (data: NodeTagMutationPayload) => void;
}

function TagEditModal({
  isOpen,
  onOpenChange,
  tag,
  onSave,
}: TagEditModalProps) {
  const [name, setName] = useState("");
  const [color, setColor] = useState("#6b7280");

  useEffect(() => {
    if (tag) {
      setName(tag.name);
      setColor(tag.color);
    } else {
      setName("");
      setColor("#6b7280");
    }
  }, [tag, isOpen]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      toast.error("标签名称不能为空");
      return;
    }

    onSave({ name, color });
  };

  const presetColors = [
    "#6b7280",
    "#ef4444",
    "#22c55e",
    "#f59e0b",
    "#3b82f6",
    "#8b5cf6",
    "#ec4899",
    "#06b6d4",
  ];

  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalContent>
        <form onSubmit={handleSubmit}>
          <ModalHeader>{tag ? "编辑标签" : "创建标签"}</ModalHeader>
          <ModalBody>
            <div className="space-y-4">
              <div>
                <label className="mb-1 block text-sm font-medium">
                  标签名称 *
                </label>
                <Input
                  required
                  placeholder="输入标签名称"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>

              <div>
                <label className="mb-1 block text-sm font-medium">颜色</label>
                <div className="mb-2 flex flex-wrap gap-2">
                  {presetColors.map((presetColor) => (
                    <button
                      key={presetColor}
                      className={`h-8 w-8 rounded border-2 ${
                        color === presetColor
                          ? "border-gray-900"
                          : "border-transparent"
                      }`}
                      style={{ backgroundColor: presetColor }}
                      type="button"
                      onClick={() => setColor(presetColor)}
                    />
                  ))}
                </div>
                <div className="flex items-center gap-2">
                  <input
                    className="h-10 w-10 cursor-pointer rounded border"
                    type="color"
                    value={color}
                    onChange={(e) => setColor(e.target.value)}
                  />
                  <Input
                    className="flex-1"
                    value={color}
                    onChange={(e) => setColor(e.target.value)}
                  />
                </div>
              </div>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button
              type="button"
              variant="flat"
              onPress={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button color="primary" type="submit">
              保存
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
}
