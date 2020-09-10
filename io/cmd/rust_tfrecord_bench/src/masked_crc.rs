use crc::crc32;

const CRC_MASK_DELTA: u32 = 0xa282ead8;

/// A Castagnoli CRC-32 checksum after a masking permutation. This is the checksum format used by
/// TFRecords.
#[derive(Debug, Copy, Clone, PartialEq, Eq)]
pub struct MaskedCrc(pub u32);

/// Applies a masking operation to an unmasked CRC.
fn mask_crc(crc: u32) -> MaskedCrc {
    MaskedCrc(((crc >> 15) | (crc << 17)).wrapping_add(CRC_MASK_DELTA))
}

/// Computes a `MaskedCrc` (see docs on that type).
pub fn compute_crc(bytes: &[u8]) -> MaskedCrc {
    mask_crc(crc32::checksum_castagnoli(bytes))
}
